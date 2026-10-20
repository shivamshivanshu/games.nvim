package room

import (
	"cmp"
	"log/slog"
	"maps"
	"slices"
	"time"

	"games.nvim/server/game"
	"games.nvim/server/internal/protocol"
	"games.nvim/server/internal/store"
	"games.nvim/server/internal/xslices"
)

// timeoutRetryDelay paces retries after a timeout fails to persist, so a flag
// that is already down does not re-fire in a tight loop.
const timeoutRetryDelay = time.Second

func (s roomState) clockRunning(g setup) bool {
	return g.clock.Enabled && len(s.acted) == len(g.Players) && !s.finished()
}

func (s roomState) charged(g setup, at time.Time) map[game.UserID]time.Duration {
	remaining := maps.Clone(s.remaining)
	if !s.clockRunning(g) {
		return remaining
	}
	elapsed := at.Sub(s.since)
	for _, user := range s.toAct(g) {
		remaining[user] = max(0, remaining[user]-elapsed)
	}
	return remaining
}

func (s roomState) soonestFlag(g setup, at time.Time) (game.UserID, time.Duration, bool) {
	users := s.toAct(g)
	if !s.clockRunning(g) || len(users) == 0 {
		return "", 0, false
	}
	remaining := s.charged(g, at)
	user := slices.MinFunc(users, func(a, b game.UserID) int { return cmp.Compare(remaining[a], remaining[b]) })
	return user, remaining[user], true
}

func (s roomState) wireClock(g setup, at time.Time) *protocol.WireClock {
	if !g.clock.Enabled {
		return nil
	}
	wire := &protocol.WireClock{RemainingMS: xslices.MapValues(s.charged(g, at), time.Duration.Milliseconds)}
	if users := s.toAct(g); s.clockRunning(g) && len(users) > 0 {
		wire.Running = &users[0]
	}
	return wire
}

func (r *Room) armTimer() {
	r.stopTimer()
	if _, d, ok := r.cur.soonestFlag(r.setup, r.setup.Time.Now()); ok {
		r.armIn(d)
	}
}

func (r *Room) armIn(d time.Duration) {
	r.stopTimer()
	r.timer = r.setup.Time.AfterFunc(d, r.tick)
}

func (r *Room) stopTimer() {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}

// tick only wakes the actor: the loop expires flagged players before every message.
func (r *Room) tick() {
	select {
	case r.inbox <- func() {}:
	case <-r.exited:
	}
}

func (r *Room) expireFlagged() {
	now := r.setup.Time.Now()
	user, left, ok := r.cur.soonestFlag(r.setup, now)
	if !ok || left > 0 {
		return
	}
	if _, err := r.commit(step{seq: r.cur.seq + 1, user: user, kind: store.StepTimeout, at: now}); err != nil {
		slog.Error("room: persist timeout", "game", r.setup.ID, "user", user, "err", err)
		r.armIn(timeoutRetryDelay)
	}
}
