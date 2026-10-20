package hub

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"slices"
	"sync"

	"games.nvim/server/game"
	"games.nvim/server/internal/auth"
	"games.nvim/server/internal/challenge"
	"games.nvim/server/internal/clock"
	"games.nvim/server/internal/limits"
	"games.nvim/server/internal/protocol"
	"games.nvim/server/internal/session"
	"games.nvim/server/internal/store"
)

type Deps struct {
	Store   *store.Store
	Games   []game.Game
	Limits  limits.Limits
	Time    clock.Clock
	Lockout *auth.Lockout
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
	h := &Hub{
		ctx:       ctx,
		deps:      d,
		gameTypes: make(map[game.GameType]game.Game, len(d.Games)),
		sessions:  make(map[game.UserID]sessionSet),
		live:      make(map[game.GameID]*liveGame),
	}
	for _, g := range d.Games {
		h.gameTypes[g.Info().Type] = g
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
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.sessions[s.User()]
	delete(set, s)
	if len(set) == 0 {
		delete(h.sessions, s.User())
	}
}

func (h *Hub) online(user game.UserID) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.sessions[user]) > 0
}

// SendUpdate runs on a room's actor goroutine, so it only queues frames.
func (h *Hub) SendUpdate(to game.UserID, u protocol.GameUpdate) {
	h.touch(u.GameID)
	h.push(protocol.TypeGameUpdate, u, to)
}

func (h *Hub) challengeChanged(c challenge.Challenge) {
	h.push(protocol.TypeChallengeChanged, wireChallenge(c), append([]game.UserID{c.From}, c.To...)...)
}

func (h *Hub) push(typ protocol.MessageType, body any, to ...game.UserID) {
	raw, err := json.Marshal(body)
	if err != nil {
		slog.Error("hub: encode push", "type", typ, "err", err)
		return
	}
	env := protocol.Envelope{Type: typ, Body: raw}
	for _, s := range h.sessionsOf(to) {
		s.Push(env)
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

func wireChallenge(c challenge.Challenge) protocol.Challenge {
	return protocol.Challenge{
		ChallengeID: c.ID,
		From:        c.From,
		To:          c.To,
		Accepted:    c.Accepted,
		Game:        c.Game,
		Options:     c.Options,
		State:       c.State,
		By:          c.By,
		GameID:      c.GameID,
		ExpiresAt:   c.ExpiresAt,
	}
}
