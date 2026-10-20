package challenge

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"slices"
	"sync"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/clock"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
)

const idBytes = 8

type StartFunc func(protocol.Challenge) (game.GameID, error)

type pendingChallenge struct {
	protocol.Challenge
	expiry clock.Timer
}

// Registry never holds its mutex while calling onChange or start, because the
// hub's callbacks may call back into it.
type Registry struct {
	ttl      time.Duration
	clock    clock.Clock
	onChange func(protocol.Challenge)
	start    StartFunc

	mu      sync.Mutex
	pending map[protocol.ChallengeID]*pendingChallenge
}

func NewRegistry(ttl time.Duration, clock clock.Clock, onChange func(protocol.Challenge), start StartFunc) *Registry {
	return &Registry{
		ttl:      ttl,
		clock:    clock,
		onChange: onChange,
		start:    start,
		pending:  make(map[protocol.ChallengeID]*pendingChallenge),
	}
}

func (r *Registry) Create(from game.UserID, to []game.UserID, info game.Info, opts game.Options) (protocol.Challenge, error) {
	if err := validateInvitees(from, to, info.Players); err != nil {
		return protocol.Challenge{}, err
	}
	c := &pendingChallenge{Challenge: protocol.Challenge{
		ChallengeID: newID(),
		From:        from,
		To:          slices.Clone(to),
		Accepted:    []game.UserID{},
		Game:        info.Type,
		Options:     opts,
		State:       protocol.ChallengePending,
		ExpiresAt:   protocol.UnixMSOf(r.clock.Now().Add(r.ttl)),
	}}
	r.mu.Lock()
	r.pending[c.ChallengeID] = c
	c.expiry = r.clock.AfterFunc(r.ttl, func() { r.expire(c.ChallengeID) })
	snapshot := c.snapshot()
	r.mu.Unlock()

	r.onChange(snapshot)
	return snapshot, nil
}

func (r *Registry) Accept(id protocol.ChallengeID, by game.UserID) error {
	changed, err := locked(&r.mu, func() (*protocol.Challenge, error) { return r.acceptLocked(id, by) })
	if err == nil && changed != nil && allAccepted(*changed) {
		changed = new(r.started(*changed))
	}
	return r.announce(changed, err)
}

func (r *Registry) Decline(id protocol.ChallengeID, by game.UserID) error {
	return r.announce(locked(&r.mu, func() (*protocol.Challenge, error) {
		c, err := r.invitedLocked(id, by)
		if err != nil {
			return nil, err
		}
		return r.closeLocked(c, protocol.ChallengeDeclined, &by), nil
	}))
}

func (r *Registry) Cancel(id protocol.ChallengeID, by game.UserID) error {
	return r.announce(locked(&r.mu, func() (*protocol.Challenge, error) {
		c, ok := r.pending[id]
		if !ok {
			return nil, unknownChallenge(id)
		}
		if by != c.From {
			return nil, protocol.BadRequestf("only the challenger can cancel a challenge")
		}
		return r.closeLocked(c, protocol.ChallengeCancelled, &by), nil
	}))
}

func (r *Registry) Pending(user game.UserID) []protocol.Challenge {
	r.mu.Lock()
	out := []protocol.Challenge{}
	for _, c := range r.pending {
		if c.From == user || slices.Contains(c.To, user) {
			out = append(out, c.snapshot())
		}
	}
	r.mu.Unlock()

	slices.SortFunc(out, func(a, b protocol.Challenge) int {
		return cmp.Or(cmp.Compare(a.ExpiresAt, b.ExpiresAt), cmp.Compare(a.ChallengeID, b.ChallengeID))
	})
	return out
}

func (r *Registry) expire(id protocol.ChallengeID) {
	_ = r.announce(locked(&r.mu, func() (*protocol.Challenge, error) {
		c, ok := r.pending[id]
		if !ok {
			return nil, nil
		}
		return r.closeLocked(c, protocol.ChallengeExpired, nil), nil
	}))
}

// started runs the game for a fully accepted challenge. The challenge is
// already out of the pending map, so no other caller can start it again.
func (r *Registry) started(c protocol.Challenge) protocol.Challenge {
	gameID, err := r.start(c)
	if err != nil {
		c.State = protocol.ChallengeCancelled
		return c
	}
	c.State, c.GameID = protocol.ChallengeAccepted, &gameID
	return c
}

func (r *Registry) announce(changed *protocol.Challenge, err error) error {
	if err != nil || changed == nil {
		return err
	}
	r.onChange(*changed)
	return nil
}

func locked[T any](mu *sync.Mutex, fn func() (T, error)) (T, error) {
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

func (r *Registry) acceptLocked(id protocol.ChallengeID, by game.UserID) (*protocol.Challenge, error) {
	c, err := r.invitedLocked(id, by)
	if err != nil {
		return nil, err
	}
	if slices.Contains(c.Accepted, by) {
		return nil, nil
	}
	c.Accepted = append(c.Accepted, by)
	if allAccepted(c.Challenge) {
		r.resolveLocked(c)
	}
	return new(c.snapshot()), nil
}

func allAccepted(c protocol.Challenge) bool { return len(c.Accepted) == len(c.To) }

func (r *Registry) invitedLocked(id protocol.ChallengeID, user game.UserID) (*pendingChallenge, error) {
	c, ok := r.pending[id]
	if !ok {
		return nil, unknownChallenge(id)
	}
	if !slices.Contains(c.To, user) {
		return nil, protocol.BadRequestf("you are not invited to this challenge")
	}
	return c, nil
}

func (r *Registry) closeLocked(c *pendingChallenge, state protocol.ChallengeState, by *game.UserID) *protocol.Challenge {
	r.resolveLocked(c)
	c.State, c.By = state, by
	return new(c.snapshot())
}

func (r *Registry) resolveLocked(c *pendingChallenge) {
	delete(r.pending, c.ChallengeID)
	c.expiry.Stop()
}

func (c *pendingChallenge) snapshot() protocol.Challenge {
	s := c.Challenge
	s.To = slices.Clone(c.To)
	s.Accepted = slices.Clone(c.Accepted)
	return s
}

func validateInvitees(from game.UserID, to []game.UserID, players game.PlayerRange) error {
	if len(to) == 0 {
		return protocol.BadRequestf("challenge needs at least one opponent")
	}
	if n := len(to) + 1; n < players.Min || n > players.Max {
		return protocol.BadRequestf("this game takes %d to %d players, got %d", players.Min, players.Max, n)
	}
	seen := map[game.UserID]bool{from: true}
	for _, u := range to {
		if u == from {
			return protocol.BadRequestf("you cannot challenge yourself")
		}
		if seen[u] {
			return protocol.BadRequestf("%s is invited twice", u)
		}
		seen[u] = true
	}
	return nil
}

func unknownChallenge(id protocol.ChallengeID) error {
	return protocol.BadRequestf("no pending challenge %s", id)
}

func newID() protocol.ChallengeID {
	b := make([]byte, idBytes)
	_, _ = rand.Read(b)
	return protocol.ChallengeID(hex.EncodeToString(b))
}
