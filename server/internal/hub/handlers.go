package hub

import (
	"context"
	"errors"

	"games.nvim/server/game"
	"games.nvim/server/internal/auth"
	"games.nvim/server/internal/protocol"
	"games.nvim/server/internal/session"
	"games.nvim/server/internal/store"
	"games.nvim/server/internal/xslices"
)

var (
	errInvalidCredentials = protocol.Error{Code: protocol.CodeAuthFailed, Msg: "authentication failed: invalid credentials"}
	errLockedOut          = protocol.Error{Code: protocol.CodeAuthFailed, Msg: "authentication failed: locked out, retry later"}
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
		protocol.TypeCatalogList:      handle(h.catalog),
		protocol.TypeUserList:         handle(h.users),
		protocol.TypeChallengeCreate:  handle(h.createChallenge),
		protocol.TypeChallengeAccept:  resolveChallenge(h.challenges.Accept),
		protocol.TypeChallengeDecline: resolveChallenge(h.challenges.Decline),
		protocol.TypeChallengeCancel:  resolveChallenge(h.challenges.Cancel),
		protocol.TypeChallengeList:    handle(h.pendingChallenges),
		protocol.TypeGameList:         handle(h.listGames),
		protocol.TypeGameSnapshot:     handle(h.gameSnapshot),
		protocol.TypeGameAct:          handle(h.act),
		protocol.TypeGameControl:      handle(h.control),
		protocol.TypeStatsGet:         handle(h.stats),
	}
}

func (h *Hub) catalog(context.Context, *session.Session, protocol.Empty) (protocol.CatalogResult, error) {
	return protocol.CatalogResult{Games: xslices.Map(h.deps.Games, func(g game.Game) protocol.GameInfoWire {
		return protocol.InfoToWire(g.Info())
	})}, nil
}

func (h *Hub) users(ctx context.Context, _ *session.Session, _ protocol.Empty) (protocol.UserListResult, error) {
	users, err := h.deps.Store.ListUsers(ctx)
	if err != nil {
		return protocol.UserListResult{}, err
	}
	return protocol.UserListResult{Users: xslices.Map(users, func(user game.UserID) protocol.UserPresence {
		return protocol.UserPresence{User: user, Online: h.online(user)}
	})}, nil
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
			return protocol.ChallengeCreateResult{}, protocol.BadRequestf("unknown user %q", user)
		}
	}
	opts, err := g.ResolveOptions(req.Options)
	if err != nil {
		return protocol.ChallengeCreateResult{}, err
	}
	c, err := h.challenges.Create(s.User(), req.To, g.Info(), opts)
	if err != nil {
		return protocol.ChallengeCreateResult{}, err
	}
	return protocol.ChallengeCreateResult{ChallengeID: c.ChallengeID}, nil
}

// handle reports the store's not-found as the client's not_found, so session
// need not know the store.
func handle[Req, Resp any](fn func(ctx context.Context, s *session.Session, req Req) (Resp, error)) session.Handler {
	return session.Handle(func(ctx context.Context, s *session.Session, req Req) (Resp, error) {
		resp, err := fn(ctx, s, req)
		if errors.Is(err, store.ErrNotFound) {
			return resp, protocol.Error{Code: protocol.CodeNotFound, Msg: err.Error()}
		}
		return resp, err
	})
}

func resolveChallenge(resolve func(protocol.ChallengeID, game.UserID) error) session.Handler {
	return handle(func(_ context.Context, s *session.Session, req protocol.ChallengeRef) (protocol.Empty, error) {
		return protocol.Empty{}, resolve(req.ChallengeID, s.User())
	})
}

func (h *Hub) pendingChallenges(_ context.Context, s *session.Session, _ protocol.Empty) (protocol.ChallengeListResult, error) {
	return protocol.ChallengeListResult{Challenges: h.challenges.Pending(s.User())}, nil
}

func (h *Hub) listGames(ctx context.Context, s *session.Session, req protocol.GameListRequest) (protocol.GameListResult, error) {
	var games []protocol.GameUpdate
	var err error
	switch req.Status {
	case "", protocol.StatusOngoing:
		games, err = h.ongoingGames(ctx, s.User())
	case protocol.StatusFinished:
		games, err = h.finishedGames(ctx, s.User())
	default:
		err = protocol.BadRequestf("unknown game status %q", req.Status)
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
	return protocol.StatsResult{
		Records: xslices.Map(records, func(r store.StatRecord) protocol.StatsRecord {
			return protocol.StatsRecord{
				User: r.User, Game: r.Game, Tally: protocol.Tally{Wins: r.Wins, Losses: r.Losses, Draws: r.Draws},
			}
		}),
		HeadToHead: xslices.Map(headToHead, func(r store.HeadToHeadRecord) protocol.HeadToHeadRecord {
			return protocol.HeadToHeadRecord{
				User: r.User, Opponent: r.Opponent, Game: r.Game, Tally: protocol.Tally{Wins: r.Wins, Losses: r.Losses, Draws: r.Draws},
			}
		}),
	}, nil
}
