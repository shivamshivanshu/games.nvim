package room

import (
	"cmp"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

// timeoutRetryDelay paces retries after a timeout fails to persist, so a flag
// that is already down does not re-fire in a tight loop.
const timeoutRetryDelay = time.Second

func (s roomState) left(user game.UserID, at time.Time) time.Duration {
	if !s.clockRunning || !s.hasTurn(user) {
		return s.remaining[user]
	}
	return max(0, s.remaining[user]-at.Sub(s.since))
}

func (s roomState) charged(at time.Time) map[game.UserID]time.Duration {
	remaining := maps.Clone(s.remaining)
	for user := range remaining {
		remaining[user] = s.left(user, at)
	}
	return remaining
}

// clockAfter is the clock once st has taken s to next: st's time is charged
// to whoever was to act, then st's own adjustment applies.
func (s roomState) clockAfter(g Setup, st step, next roomState) map[game.UserID]time.Duration {
	remaining := s.charged(st.at)
	switch {
	case st.kind == store.StepAction && s.clockRunning && s.hasTurn(st.user) && !next.hasTurn(st.user):
		remaining[st.user] += g.clock.Increment
	case st.kind == store.StepTimeout && remaining != nil:
		remaining[st.user] = 0
	}
	return remaining
}

func (s roomState) soonestFlag(at time.Time) (game.UserID, time.Duration, bool) {
	if !s.clockRunning || len(s.toAct) == 0 {
		return "", 0, false
	}
	user := slices.MinFunc(s.toAct, func(a, b game.UserID) int { return cmp.Compare(s.left(a, at), s.left(b, at)) })
	return user, s.left(user, at), true
}

func (s roomState) wireClock(g Setup, at time.Time) *protocol.WireClock {
	if !g.clock.Enabled {
		return nil
	}
	wire := &protocol.WireClock{RemainingMS: xslices.MapValues(s.charged(at), time.Duration.Milliseconds), AsOfMS: protocol.UnixMSOf(at)}
	if s.clockRunning && len(s.toAct) > 0 {
		running := s.toAct[0]
		wire.Running = &running
	}
	return wire
}

func (r *Room) armTimer() {
	r.stopTimer()
	if _, d, ok := r.cur.soonestFlag(r.setup.Time.Now()); ok {
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
	user, left, ok := r.cur.soonestFlag(now)
	if !ok || left > 0 {
		return
	}
	if _, err := r.commit(step{seq: r.cur.seq + 1, user: user, kind: store.StepTimeout, at: now}); err != nil {
		slog.Error("room: persist timeout", "game", r.setup.ID, "user", user, "err", err)
		r.armIn(timeoutRetryDelay)
	}
}
