package hub

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/auth"
	"github.com/shivamshivanshu/games.nvim/server/internal/challenge"
	"github.com/shivamshivanshu/games.nvim/server/internal/clock"
	"github.com/shivamshivanshu/games.nvim/server/internal/limits"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/session"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

// Deps is what a Hub runs on. Seeds draws each new game's seed; nil draws
// from crypto/rand.
type Deps struct {
	Store   *store.Store
	Games   []game.Game
	Limits  limits.Limits
	Time    clock.Clock
	Lockout *auth.Lockout
	Seeds   func() game.Seed
}

type sessionSet map[*session.Session]struct{}

// Hub never holds mu while calling into a Room: rooms call SendUpdate, which
// takes mu, from their actor goroutine.
type Hub struct {
	ctx        context.Context
	deps       Deps
	gameTypes  map[game.GameType]game.Game
	challenges *challenge.Registry

	mu       sync.Mutex
	sessions map[game.UserID]sessionSet
	live     map[game.GameID]*liveGame
}

// New restores every ongoing game from the store. Cancelling ctx stops all rooms.
func New(ctx context.Context, d Deps) (*Hub, error) {
	if d.Seeds == nil {
		d.Seeds = newSeed
	}
	h := &Hub{
		ctx:       ctx,
		deps:      d,
		gameTypes: make(map[game.GameType]game.Game, len(d.Games)),
		sessions:  make(map[game.UserID]sessionSet),
		live:      make(map[game.GameID]*liveGame),
	}
	for _, g := range d.Games {
		info := g.Info()
		if err := game.CheckGame(g); err != nil {
			return nil, fmt.Errorf("hub: %w", err)
		}
		if _, registered := h.gameTypes[info.Type]; registered {
			return nil, fmt.Errorf("hub: game %s registered twice", info.Type)
		}
		h.gameTypes[info.Type] = g
	}
	h.challenges = challenge.NewRegistry(d.Limits.ChallengeTTL, d.Time, h.challengeChanged, h.startChallenge)
	if err := h.restoreOngoing(ctx); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *Hub) Hooks() session.Hooks {
	return session.Hooks{OnAuthenticated: h.addSession, OnClosed: h.removeSession}
}

func (h *Hub) addSession(s *session.Session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.sessions[s.User()]
	if !ok {
		set = sessionSet{}
		h.sessions[s.User()] = set
	}
	set[s] = struct{}{}
}

func (h *Hub) removeSession(s *session.Session) {
	if h.dropSession(s) {
		h.pauseGamesOf(s.User())
	}
}

// dropSession reports whether s was its user's last session.
func (h *Hub) dropSession(s *session.Session) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.sessions[s.User()]
	delete(set, s)
	if len(set) > 0 {
		return false
	}
	delete(h.sessions, s.User())
	return true
}

// pauseGamesOf pauses the real-time games of a user who went away, so they
// do not play on without the user; each room drops the pause if the user is
// back by the time it gets there. Rooms already take mu from their actor (see
// SendUpdate), so asking online there keeps the lock order.
func (h *Hub) pauseGamesOf(user game.UserID) {
	away := h.away(user)
	for _, r := range h.liveRoomsOf(user) {
		if err := r.Pause(h.ctx, away); err != nil && h.ctx.Err() == nil {
			slog.Error("hub: pause on disconnect", "user", user, "err", err)
		}
	}
}

// away reports whether any of users has no session left.
func (h *Hub) away(users ...game.UserID) func() bool {
	return func() bool {
		return slices.ContainsFunc(users, func(u game.UserID) bool { return !h.online(u) })
	}
}

func (h *Hub) online(user game.UserID) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.sessions[user]) > 0
}

// SendUpdate runs on a room's actor goroutine, so it only queues frames.
func (h *Hub) SendUpdate(to game.UserID, u protocol.GameUpdate) {
	h.push(protocol.TypeGameUpdate, u, to)
}

func (h *Hub) challengeChanged(c protocol.Challenge) {
	h.push(protocol.TypeChallengeChanged, c, c.Participants()...)
}

func (h *Hub) push(typ protocol.MessageType, body any, to ...game.UserID) {
	sessions := h.sessionsOf(to)
	if len(sessions) == 0 {
		return
	}
	frame, err := protocol.EncodeFrame(nil, typ, body)
	if err != nil {
		slog.Error("hub: encode push", "type", typ, "err", err)
		return
	}
	for _, s := range sessions {
		s.Push(frame)
	}
}

func (h *Hub) sessionsOf(users []game.UserID) []*session.Session {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []*session.Session
	for _, user := range users {
		out = slices.AppendSeq(out, maps.Keys(h.sessions[user]))
	}
	return out
}
