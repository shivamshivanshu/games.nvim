package room

import (
	"context"
	"fmt"
	"maps"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

func Restore(ctx context.Context, g Setup, steps []store.StepRecord) (*Room, error) {
	p := g.Params
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
		st, err := stepOf(rec)
		if err != nil {
			return nil, fmt.Errorf("room: restore %s: replay seq %d: %w", p.ID, rec.Seq, err)
		}
		if s, err = s.apply(g, st); err != nil {
			return nil, fmt.Errorf("room: restore %s: replay seq %d: %w", p.ID, rec.Seq, err)
		}
	}
	if len(steps) > 0 && g.clock.Enabled {
		s.remaining = steps[len(steps)-1].ClockRemaining
	}
	// Downtime is not charged: clocks resume from the last persisted values as of now.
	s.since = p.Time.Now()
	r := newRoom(ctx, g, s)
	// Nobody is connected to a game the server has just restored, so a
	// ticking one waits, paused, for its player.
	if err := r.pause(); err != nil {
		return nil, fmt.Errorf("room: restore %s: pause: %w", p.ID, err)
	}
	go r.run()
	return r, nil
}
