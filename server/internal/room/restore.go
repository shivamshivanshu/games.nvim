package room

import (
	"context"
	"fmt"
	"maps"

	"games.nvim/server/game"
	"games.nvim/server/internal/store"
)

// Restore rebuilds a room from its persisted steps. p.Players[0] must be the
// challenger, since seat assignment in Game.New depends on who challenged.
func Restore(ctx context.Context, p Params, steps []store.StepRecord) (*Room, error) {
	if len(p.Players) == 0 {
		return nil, fmt.Errorf("room: restore %s: no players", p.ID)
	}
	g := newSetup(p)
	invitees := make([]game.UserID, 0, len(p.Players)-1)
	for _, player := range p.Players[1:] {
		invitees = append(invitees, player.User)
	}
	state, seats, err := p.Game.New(p.Options, p.Players[0].User, invitees, game.StepRNG(p.Seed, 0))
	if err != nil {
		return nil, fmt.Errorf("room: restore %s: %w", p.ID, err)
	}
	if !maps.Equal(seats, g.seats) {
		return nil, fmt.Errorf("room: restore %s: replayed seats %v do not match stored %v", p.ID, seats, g.seats)
	}
	s := initialState(g, state, p.Time.Now())
	for i, rec := range steps {
		if rec.Seq != uint64(i)+1 {
			return nil, fmt.Errorf("room: restore %s: step %d has seq %d", p.ID, i+1, rec.Seq)
		}
		if s, err = s.apply(g, rec); err != nil {
			return nil, fmt.Errorf("room: restore %s: replay seq %d: %w", p.ID, rec.Seq, err)
		}
	}
	if len(steps) > 0 && g.Clock.Enabled {
		s.remaining = fromMS(steps[len(steps)-1].ClockRemainingMS)
	}
	// Downtime is not charged: clocks resume from the last persisted values as of now.
	s.since = p.Time.Now()
	return start(ctx, g, s), nil
}
