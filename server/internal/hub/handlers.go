package hub

import (
	"context"
	"errors"
	"fmt"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/auth"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/room"
	"github.com/shivamshivanshu/games.nvim/server/internal/session"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

var (
	errInvalidCredentials = protocol.Error{Code: protocol.CodeAuthFailed, Msg: "authentication failed: invalid credentials"}
	errLockedOut          = protocol.Error{Code: protocol.CodeAuthFailed, Msg: "authentication failed: locked out, retry later"}
	errSignupDisabled     = protocol.Error{Code: protocol.CodeSignupDisabled, Msg: "sign-up is disabled on this server"}
	errPassphraseRejected = protocol.Error{Code: protocol.CodePassphraseRejected, Msg: "sign-up passphrase rejected"}
)

func (h *Hub) Authenticator() session.Authenticator {
	return session.Authenticator{Login: h.login, Register: h.register}
}

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

// register checks the passphrase before anything else about the account, so
// a client without it learns nothing of which names are taken.
func (h *Hub) register(ctx context.Context, s *session.Session, req protocol.RegisterRequest) (protocol.LoginResult, error) {
	if h.deps.Lockout.Locked(s.RemoteAddr()) {
		return protocol.LoginResult{}, errLockedOut
	}
	if h.deps.SignupPassphrase == "" {
		return protocol.LoginResult{}, errSignupDisabled
	}
	if !auth.PassphraseMatches(h.deps.SignupPassphrase, req.Passphrase) {
		h.deps.Lockout.RecordFailure(s.RemoteAddr())
		return protocol.LoginResult{}, errPassphraseRejected
	}
	err := auth.CreateAccount(ctx, h.deps.Store, req.Username, req.Password, h.deps.PasswordCost)
	if errors.Is(err, store.ErrUserExists) {
		return protocol.LoginResult{}, protocol.Error{Code: protocol.CodeUserExists, Msg: fmt.Sprintf("user %q already exists", req.Username)}
	}
	if err != nil {
		return protocol.LoginResult{}, err
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
		protocol.TypeGameStart:        session.Handle(h.startSolo),
		protocol.TypeStatsGet:         session.Handle(h.stats),
		protocol.TypeGameVerify:       session.Handle(h.verify),
	}
}

func (h *Hub) catalog(context.Context, *session.Session, protocol.Empty) (protocol.CatalogResult, error) {
	return protocol.CatalogOf(h.deps.Games), nil
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
	g, opts, err := h.resolveGame(req.Game, req.Options)
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
	c, err := h.challenges.Create(s.User(), req.To, g.Info(), opts)
	if err != nil {
		return protocol.ChallengeCreateResult{}, err
	}
	return protocol.ChallengeCreateResult{ChallengeID: c.ChallengeID}, nil
}

func (h *Hub) startSolo(_ context.Context, s *session.Session, req protocol.GameStartRequest) (protocol.GameUpdate, error) {
	g, opts, err := h.resolveGame(req.Game, req.Options)
	if err != nil {
		return protocol.GameUpdate{}, err
	}
	if !g.Info().Players.AllowsSolo() {
		return protocol.GameUpdate{}, protocol.BadRequestf("%s is not a solo game: challenge an opponent", req.Game)
	}
	id, first, err := h.startGame(req.Game, opts, s.User(), nil)
	if err != nil {
		return protocol.GameUpdate{}, err
	}
	update, ok := first[s.User()]
	if !ok {
		return protocol.GameUpdate{}, fmt.Errorf("hub: solo game %s has no first update", id)
	}
	return update, nil
}

func resolveChallenge(resolve func(protocol.ChallengeID, game.UserID) error) session.Handler {
	return session.Handle(func(_ context.Context, s *session.Session, req protocol.ChallengeRef) (protocol.Empty, error) {
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
	return h.submit(req.GameID, func(r *room.Room) (game.Seq, error) { return r.Act(ctx, s.User(), req.Seq, req.Action) })
}

func (h *Hub) control(ctx context.Context, s *session.Session, req protocol.GameControlRequest) (protocol.SeqResult, error) {
	return h.submit(req.GameID, func(r *room.Room) (game.Seq, error) { return r.Control(ctx, s.User(), req.Seq, req.Control) })
}

// submit takes a step on the live room of id and gives the seq it reached.
func (h *Hub) submit(id game.GameID, step func(*room.Room) (game.Seq, error)) (protocol.SeqResult, error) {
	r, err := h.liveRoom(id)
	if err != nil {
		return protocol.SeqResult{}, err
	}
	seq, err := step(r)
	return protocol.SeqResult{Seq: seq}, err
}

func (h *Hub) stats(ctx context.Context, _ *session.Session, req protocol.StatsRequest) (protocol.StatsResult, error) {
	return h.deps.Store.Stats(ctx, req.User)
}
