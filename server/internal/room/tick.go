package room

import (
	"context"
	"log/slog"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

// tickDue is when state, reached at from, wants its next tick: zero when it
// wants none.
func tickDue(state game.ErasedState, from time.Time) time.Time {
	d, ticking := state.NextTick()
	if !ticking {
		return time.Time{}
	}
	return from.Add(d)
}

// tickAfter is when next, which st took s to, is due its tick. A due tick
// keeps its time across the steps players make, so input never holds the
// game back.
func (s roomState) tickAfter(st step, next roomState) time.Time {
	if next.finished() {
		return time.Time{}
	}
	due := tickDue(next.game, st.at)
	if st.kind != store.StepTick && s.ticking() && !due.IsZero() {
		return s.nextTickAt
	}
	return due
}

func (s roomState) applyTick(g Setup, st step) (roomState, error) {
	if s.finished() {
		return s, errGameOver
	}
	state, events, err := s.game.Tick(game.StepRNG(g.Seed, st.seq))
	if err != nil {
		return s, err
	}
	s.game, s.events = state, events
	return s.withKnockouts(g).settled(g), nil
}

func (s roomState) applyPause(g Setup) roomState {
	s.game = s.game.Pause()
	return s.settled(g)
}

func (r *Room) tickIfDue() {
	now := r.setup.Time.Now()
	if !r.cur.ticking() || now.Before(r.cur.nextTickAt) {
		return
	}
	if _, err := r.commit(step{seq: r.cur.seq + 1, kind: store.StepTick, at: now}); err != nil {
		slog.Error("room: persist tick", "game", r.setup.ID, "err", err)
		r.armIn(retryDelay)
	}
}

// Pause stops a ticking game until its player acts again, as the hub does
// when the player goes away. away is asked on the room's actor, where steps
// are ordered: a pause that comes after the player is back is dropped, so it
// cannot stop a game they have just resumed. A game that is not ticking is
// left as it is.
func (r *Room) Pause(ctx context.Context, away func() bool) error {
	_, err := ask(ctx, r, func() (game.Seq, error) {
		if !away() {
			return r.cur.seq, nil
		}
		return r.pause()
	}, func() (game.Seq, error) { return 0, nil })
	return err
}

func (r *Room) pause() (game.Seq, error) {
	if !r.cur.ticking() {
		return r.cur.seq, nil
	}
	return r.commit(step{seq: r.cur.seq + 1, kind: store.StepPause, at: r.setup.Time.Now()})
}
