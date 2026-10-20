package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"games.nvim/server/game"
	"games.nvim/server/internal/auth"
	"games.nvim/server/internal/protocol"
	"games.nvim/server/internal/session"
	"games.nvim/server/internal/store"
)

var (
	errInvalidCredentials = fmt.Errorf("%w: invalid credentials", session.ErrAuthFailed)
	errLockedOut          = fmt.Errorf("%w: locked out, retry later", session.ErrAuthFailed)
)

func (h *Hub) Login() session.Authenticator { return h.login }

func (h *Hub) login(ctx context.Context, s *session.Session, req protocol.LoginRequest) (protocol.LoginResult, error) {
	if h.deps.Lockout.Locked(s.RemoteAddr()) {
		return protocol.LoginResult{}, errLockedOut
	}
	hash, err := h.deps.Store.PasswordHash(ctx, req.Username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return protocol.LoginResult{}, err
	}
	if err != nil || !auth.CheckPassword(hash, req.Password) {
		h.deps.Lockout.RecordFailure(s.RemoteAddr())
		return protocol.LoginResult{}, errInvalidCredentials
	}
	return protocol.LoginResult{User: req.Username}, nil
}

func (h *Hub) Routes() session.Router {
	return session.Router{
		protocol.TypeCatalogList:      session.Handle(h.catalog),
		protocol.TypeUserList:         session.Handle(h.users),
		protocol.TypeChallengeCreate:  session.Handle(h.createChallenge),
		protocol.TypeChallengeAccept:  resolveChallenge(h.challenges.Accept),
		protocol.TypeChallengeDecline: resolveChallenge(h.challenges.Decline),
		protocol.TypeChallengeCancel:  resolveChallenge(h.challenges.Cancel),
		protocol.TypeChallengeList:    session.Handle(h.pendingChallenges),
		protocol.TypeGameList:         session.Handle(h.listGames),
		protocol.TypeGameSnapshot:     session.Handle(h.gameSnapshot),
		protocol.TypeGameAct:          session.Handle(h.act),
		protocol.TypeGameControl:      session.Handle(h.control),
		protocol.TypeStatsGet:         session.Handle(h.stats),
	}
}

func (h *Hub) catalog(context.Context, *session.Session, protocol.Empty) (protocol.CatalogResult, error) {
	games := make([]protocol.GameInfoWire, 0, len(h.deps.Games))
	for _, g := range h.deps.Games {
		games = append(games, protocol.InfoToWire(g.Info()))
	}
	return protocol.CatalogResult{Games: games}, nil
}

func (h *Hub) users(ctx context.Context, _ *session.Session, _ protocol.Empty) (protocol.UserListResult, error) {
	users, err := h.deps.Store.ListUsers(ctx)
	if err != nil {
		return protocol.UserListResult{}, err
	}
	presence := make([]protocol.UserPresence, 0, len(users))
	for _, user := range users {
		presence = append(presence, protocol.UserPresence{User: user, Online: h.online(user)})
	}
	return protocol.UserListResult{Users: presence}, nil
}

func (h *Hub) createChallenge(ctx context.Context, s *session.Session, req protocol.ChallengeCreateRequest) (protocol.ChallengeCreateResult, error) {
	g, err := h.gameType(req.Game)
	if err != nil {
		return protocol.ChallengeCreateResult{}, err
	}
	for _, user := range req.To {
		exists, err := h.deps.Store.UserExists(ctx, user)
		if err != nil {
			return protocol.ChallengeCreateResult{}, err
		}
		if !exists {
			return protocol.ChallengeCreateResult{}, game.BadRequestError{Msg: fmt.Sprintf("unknown user %q", user)}
		}
	}
	opts, err := g.ResolveOptions(req.Options)
	if err != nil {
		return protocol.ChallengeCreateResult{}, err
	}
	encodedOpts, err := json.Marshal(opts)
	if err != nil {
		return protocol.ChallengeCreateResult{}, err
	}
	c, err := h.challenges.Create(s.User(), req.To, g.Info(), encodedOpts)
	if err != nil {
		return protocol.ChallengeCreateResult{}, err
	}
	return protocol.ChallengeCreateResult{ChallengeID: c.ID}, nil
}

func resolveChallenge(resolve func(protocol.ChallengeID, game.UserID) error) session.Handler {
	return session.Handle(func(_ context.Context, s *session.Session, req protocol.ChallengeRef) (protocol.Empty, error) {
		return protocol.Empty{}, resolve(req.ChallengeID, s.User())
	})
}

func (h *Hub) pendingChallenges(_ context.Context, s *session.Session, _ protocol.Empty) (protocol.ChallengeListResult, error) {
	pending := h.challenges.Pending(s.User())
	challenges := make([]protocol.Challenge, 0, len(pending))
	for _, c := range pending {
		challenges = append(challenges, wireChallenge(c))
	}
	return protocol.ChallengeListResult{Challenges: challenges}, nil
}

func (h *Hub) listGames(ctx context.Context, s *session.Session, req protocol.GameListRequest) (protocol.GameListResult, error) {
	var games []protocol.GameUpdate
	var err error
	switch req.Status {
	case "", protocol.StatusOngoing:
		games, err = h.ongoingGames(s.User())
	case protocol.StatusFinished:
		games, err = h.finishedGames(ctx, s.User())
	default:
		err = game.BadRequestError{Msg: fmt.Sprintf("unknown game status %q", req.Status)}
	}
	return protocol.GameListResult{Games: games}, err
}

func (h *Hub) gameSnapshot(ctx context.Context, s *session.Session, req protocol.GameSnapshotRequest) (protocol.GameUpdate, error) {
	return h.snapshot(ctx, req.GameID, s.User())
}

func (h *Hub) act(ctx context.Context, s *session.Session, req protocol.GameActRequest) (protocol.SeqResult, error) {
	r, err := h.liveRoom(req.GameID)
	if err != nil {
		return protocol.SeqResult{}, err
	}
	seq, err := r.Act(ctx, s.User(), req.Seq, req.Action)
	return protocol.SeqResult{Seq: seq}, err
}

func (h *Hub) control(ctx context.Context, s *session.Session, req protocol.GameControlRequest) (protocol.SeqResult, error) {
	r, err := h.liveRoom(req.GameID)
	if err != nil {
		return protocol.SeqResult{}, err
	}
	seq, err := r.Control(ctx, s.User(), req.Seq, req.Control)
	return protocol.SeqResult{Seq: seq}, err
}

func (h *Hub) stats(ctx context.Context, _ *session.Session, _ protocol.Empty) (protocol.StatsResult, error) {
	records, headToHead, err := h.deps.Store.Stats(ctx)
	if err != nil {
		return protocol.StatsResult{}, err
	}
	out := protocol.StatsResult{
		Records:    make([]protocol.StatsRecord, 0, len(records)),
		HeadToHead: make([]protocol.HeadToHeadRecord, 0, len(headToHead)),
	}
	for _, r := range records {
		out.Records = append(out.Records, protocol.StatsRecord{
			User: r.User, Game: r.Game, Tally: protocol.Tally{Wins: r.Wins, Losses: r.Losses, Draws: r.Draws},
		})
	}
	for _, r := range headToHead {
		out.HeadToHead = append(out.HeadToHead, protocol.HeadToHeadRecord{
			User: r.User, Opponent: r.Opponent, Game: r.Game, Tally: protocol.Tally{Wins: r.Wins, Losses: r.Losses, Draws: r.Draws},
		})
	}
	return out, nil
}
