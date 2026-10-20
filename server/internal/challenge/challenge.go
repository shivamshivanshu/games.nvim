package challenge

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"games.nvim/server/game"
	"games.nvim/server/internal/protocol"
	"games.nvim/server/internal/room"
)

const idBytes = 8

type Challenge struct {
	ID        protocol.ChallengeID
	From      game.UserID
	To        []game.UserID
	Accepted  []game.UserID
	Game      game.GameType
	Options   json.RawMessage
	State     protocol.ChallengeState
	By        *game.UserID
	GameID    *game.GameID
	ExpiresAt time.Time
}

type StartFunc func(Challenge) (game.GameID, error)

type pendingChallenge struct {
	Challenge
	expiry room.Timer
}

// Registry never holds its mutex while calling onChange or start, because the
// hub's callbacks may call back into it.
type Registry struct {
	ttl      time.Duration
	clock    room.Clock
	onChange func(Challenge)
	start    StartFunc

	mu      sync.Mutex
	pending map[protocol.ChallengeID]*pendingChallenge
}

func NewRegistry(ttl time.Duration, clock room.Clock, onChange func(Challenge), start StartFunc) *Registry {
	return &Registry{
		ttl:      ttl,
		clock:    clock,
		onChange: onChange,
		start:    start,
		pending:  make(map[protocol.ChallengeID]*pendingChallenge),
	}
}

func (r *Registry) Create(from game.UserID, to []game.UserID, info game.Info, opts json.RawMessage) (Challenge, error) {
	if err := validateInvitees(from, to, info.Players); err != nil {
		return Challenge{}, err
	}
	c := &pendingChallenge{Challenge: Challenge{
		ID:        newID(),
		From:      from,
		To:        slices.Clone(to),
		Accepted:  []game.UserID{},
		Game:      info.Type,
		Options:   opts,
		State:     protocol.ChallengePending,
		ExpiresAt: r.clock.Now().Add(r.ttl),
	}}

	r.mu.Lock()
	r.pending[c.ID] = c
	c.expiry = r.clock.AfterFunc(r.ttl, func() { r.expire(c.ID) })
	snapshot := c.snapshot()
	r.mu.Unlock()

	r.onChange(snapshot)
	return snapshot, nil
}

func (r *Registry) Accept(id protocol.ChallengeID, by game.UserID) error {
	r.mu.Lock()
	c, err := r.invitedLocked(id, by)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	if slices.Contains(c.Accepted, by) {
		r.mu.Unlock()
		return nil
	}
	c.Accepted = append(c.Accepted, by)
	if len(c.Accepted) < len(c.To) {
		snapshot := c.snapshot()
		r.mu.Unlock()
		r.onChange(snapshot)
		return nil
	}
	r.resolveLocked(c)
	snapshot := c.snapshot()
	r.mu.Unlock()

	r.onChange(r.started(snapshot))
	return nil
}

func (r *Registry) Decline(id protocol.ChallengeID, by game.UserID) error {
	r.mu.Lock()
	c, err := r.invitedLocked(id, by)
	if err != nil {
		r.mu.Unlock()
		return err
	}
	r.resolveLocked(c)
	c.State, c.By = protocol.ChallengeDeclined, &by
	snapshot := c.snapshot()
	r.mu.Unlock()

	r.onChange(snapshot)
	return nil
}

func (r *Registry) Cancel(id protocol.ChallengeID, by game.UserID) error {
	r.mu.Lock()
	c, ok := r.pending[id]
	if !ok {
		r.mu.Unlock()
		return unknownChallenge(id)
	}
	if by != c.From {
		r.mu.Unlock()
		return game.BadRequestError{Msg: "only the challenger can cancel a challenge"}
	}
	r.resolveLocked(c)
	c.State, c.By = protocol.ChallengeCancelled, &by
	snapshot := c.snapshot()
	r.mu.Unlock()

	r.onChange(snapshot)
	return nil
}

// Pending returns the open challenges a user sent or received, soonest expiry first.
func (r *Registry) Pending(user game.UserID) []Challenge {
	r.mu.Lock()
	out := []Challenge{}
	for _, c := range r.pending {
		if c.From == user || slices.Contains(c.To, user) {
			out = append(out, c.snapshot())
		}
	}
	r.mu.Unlock()

	slices.SortFunc(out, func(a, b Challenge) int {
		return cmp.Or(a.ExpiresAt.Compare(b.ExpiresAt), cmp.Compare(a.ID, b.ID))
	})
	return out
}

func (r *Registry) expire(id protocol.ChallengeID) {
	r.mu.Lock()
	c, ok := r.pending[id]
	if !ok {
		r.mu.Unlock()
		return
	}
	r.resolveLocked(c)
	c.State = protocol.ChallengeExpired
	snapshot := c.snapshot()
	r.mu.Unlock()

	r.onChange(snapshot)
}

// started runs the game for a fully accepted challenge. The challenge is
// already out of the pending map, so no other caller can start it again.
func (r *Registry) started(c Challenge) Challenge {
	gameID, err := r.start(c)
	if err != nil {
		c.State = protocol.ChallengeCancelled
		return c
	}
	c.State, c.GameID = protocol.ChallengeAccepted, &gameID
	return c
}

func (r *Registry) invitedLocked(id protocol.ChallengeID, user game.UserID) (*pendingChallenge, error) {
	c, ok := r.pending[id]
	if !ok {
		return nil, unknownChallenge(id)
	}
	if !slices.Contains(c.To, user) {
		return nil, game.BadRequestError{Msg: "you are not invited to this challenge"}
	}
	return c, nil
}

func (r *Registry) resolveLocked(c *pendingChallenge) {
	delete(r.pending, c.ID)
	c.expiry.Stop()
}

func (c *pendingChallenge) snapshot() Challenge {
	s := c.Challenge
	s.To = slices.Clone(c.To)
	s.Accepted = slices.Clone(c.Accepted)
	return s
}

func validateInvitees(from game.UserID, to []game.UserID, players game.PlayerRange) error {
	if len(to) == 0 {
		return game.BadRequestError{Msg: "challenge needs at least one opponent"}
	}
	if n := len(to) + 1; n < players.Min || n > players.Max {
		return game.BadRequestError{Msg: fmt.Sprintf("this game takes %d to %d players, got %d", players.Min, players.Max, n)}
	}
	seen := map[game.UserID]bool{from: true}
	for _, u := range to {
		if u == from {
			return game.BadRequestError{Msg: "you cannot challenge yourself"}
		}
		if seen[u] {
			return game.BadRequestError{Msg: fmt.Sprintf("%s is invited twice", u)}
		}
		seen[u] = true
	}
	return nil
}

func unknownChallenge(id protocol.ChallengeID) error {
	return game.BadRequestError{Msg: fmt.Sprintf("no pending challenge %s", id)}
}

func newID() protocol.ChallengeID {
	b := make([]byte, idBytes)
	_, _ = rand.Read(b)
	return protocol.ChallengeID(hex.EncodeToString(b))
}
