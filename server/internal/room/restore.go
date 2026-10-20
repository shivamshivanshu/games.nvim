package room

import (
	"context"
	"fmt"
	"maps"

	"games.nvim/server/game"
	"games.nvim/server/internal/store"
	"games.nvim/server/internal/xslices"
)

// Restore rebuilds a room from its persisted steps.
func Restore(ctx context.Context, p Params, steps []store.StepRecord) (*Room, error) {
	g := newSetup(p)
	state, seats, err := p.Game.New(p.Options, p.Challenger, p.invitees(), game.StepRNG(p.Seed, 0))
	if err != nil {
		return nil, fmt.Errorf("room: restore %s: %w", p.ID, err)
	}
	if !maps.Equal(seats, g.seats) {
		return nil, fmt.Errorf("room: restore %s: replayed seats %v do not match stored %v", p.ID, seats, g.seats)
	}
	s := initialState(g, state, p.Time.Now())
	for i, rec := range steps {
		if rec.Seq != game.Seq(i)+1 {
			return nil, fmt.Errorf("room: restore %s: step %d has seq %d", p.ID, i+1, rec.Seq)
		}
		if s, err = s.apply(g, rec); err != nil {
			return nil, fmt.Errorf("room: restore %s: replay seq %d: %w", p.ID, rec.Seq, err)
		}
	}
	if len(steps) > 0 && g.Clock.Enabled {
		s.remaining = xslices.MapValues(steps[len(steps)-1].ClockRemainingMS, durationFromMS)
	}
	// Downtime is not charged: clocks resume from the last persisted values as of now.
	s.since = p.Time.Now()
	return start(ctx, g, s), nil
}
