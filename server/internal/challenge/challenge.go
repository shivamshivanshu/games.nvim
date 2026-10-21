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
	r.mu.Lock()
	c, err := r.invitedLocked(id, by)
	if err != nil || slices.Contains(c.Accepted, by) {
		r.mu.Unlock()
		return err
	}
	c.Accepted = append(c.Accepted, by)
	acceptedByAll := len(c.Accepted) == len(c.To)
	if acceptedByAll {
		r.resolveLocked(c)
	}
	accepted := c.snapshot()
	r.mu.Unlock()

	if acceptedByAll {
		accepted = r.started(accepted)
	}
	r.onChange(accepted)
	return nil
}

func (r *Registry) Decline(id protocol.ChallengeID, by game.UserID) error {
	return r.close(func() (*pendingChallenge, error) { return r.invitedLocked(id, by) }, protocol.ChallengeDeclined, &by)
}

func (r *Registry) Cancel(id protocol.ChallengeID, by game.UserID) error {
	return r.close(func() (*pendingChallenge, error) {
		c, err := r.pendingLocked(id)
		if err == nil && by != c.From {
			return nil, protocol.BadRequestf("only the challenger can cancel a challenge")
		}
		return c, err
	}, protocol.ChallengeCancelled, &by)
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
	_ = r.close(func() (*pendingChallenge, error) { return r.pendingLocked(id) }, protocol.ChallengeExpired, nil)
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

// close ends the pending challenge that find picks under the lock, then
// announces it.
func (r *Registry) close(find func() (*pendingChallenge, error), state protocol.ChallengeState, by *game.UserID) error {
	r.mu.Lock()
	c, err := find()
	if err != nil {
		r.mu.Unlock()
		return err
	}
	r.resolveLocked(c)
	c.State, c.By = state, by
	closed := c.snapshot()
	r.mu.Unlock()

	r.onChange(closed)
	return nil
}

func (r *Registry) pendingLocked(id protocol.ChallengeID) (*pendingChallenge, error) {
	c, ok := r.pending[id]
	if !ok {
		return nil, unknownChallenge(id)
	}
	return c, nil
}

func (r *Registry) invitedLocked(id protocol.ChallengeID, user game.UserID) (*pendingChallenge, error) {
	c, err := r.pendingLocked(id)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(c.To, user) {
		return nil, protocol.BadRequestf("you are not invited to this challenge")
	}
	return c, nil
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
