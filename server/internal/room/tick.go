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
// game back. A tick counts the next one from when it was due, so a late tick
// does not slow the game, unless it is a whole interval or more late: then
// from when it ran, so a stall is not made up in a burst.
func (s roomState) tickAfter(st step, next roomState) time.Time {
	if next.finished() {
		return time.Time{}
	}
	if st.kind == store.StepTick {
		from := s.nextTickAt
		if due := tickDue(next.game, from); !due.IsZero() && !due.After(st.at) {
			from = st.at
		}
		return tickDue(next.game, from)
	}
	due := tickDue(next.game, st.at)
	if s.ticking() && !due.IsZero() {
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
	if !r.settlePause() {
		return
	}
	now := r.setup.Time.Now()
	if !r.cur.ticking() || now.Before(r.cur.nextTickAt) {
		return
	}
	r.commitAuto(step{seq: r.cur.seq + 1, kind: store.StepTick, at: now})
}

// Pause stops a ticking game until its player acts again, as the hub does
// when the player goes away. away is asked on the room's actor, where steps
// are ordered: a pause that comes after the player is back is dropped, so it
// cannot stop a game they have just resumed. A game that is not ticking is
// left as it is. A pause that fails to persist is kept, holding the ticks
// back, and retried as a tick is.
func (r *Room) Pause(ctx context.Context, away func() bool) error {
	_, err := ask(ctx, r, func() (game.Seq, error) {
		r.pauseWanted = away
		r.settlePause()
		return r.cur.seq, nil
	}, func() (game.Seq, error) { return 0, nil })
	return err
}

// settlePause persists the pause wanted while its player is still away, and
// reports whether none is left waiting to persist.
func (r *Room) settlePause() bool {
	if r.pauseWanted == nil {
		return true
	}
	if r.pauseWanted() && r.cur.ticking() {
		now := r.setup.Time.Now()
		if now.Before(r.pauseRetryAt) {
			r.armIn(r.pauseRetryAt.Sub(now))
			return false
		}
		if !r.commitAuto(step{seq: r.cur.seq + 1, kind: store.StepPause, at: now}) {
			r.pauseRetryAt = now.Add(retryDelay)
			return false
		}
	}
	r.pauseWanted, r.pauseRetryAt = nil, time.Time{}
	return true
}

// commitAuto commits a step the room takes on its own. One that fails to
// persist is logged and retried after retryDelay.
func (r *Room) commitAuto(st step) bool {
	if _, err := r.commit(st); err != nil {
		attrs := []any{"game", r.setup.ID}
		if st.user != "" {
			attrs = append(attrs, "user", st.user)
		}
		slog.Error("room: persist "+string(st.kind), append(attrs, "err", err)...)
		r.armIn(retryDelay)
		return false
	}
	return true
}
