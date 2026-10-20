package room

import (
	"log/slog"
	"maps"
	"time"

	"games.nvim/server/game"
	"games.nvim/server/internal/protocol"
	"games.nvim/server/internal/store"
)

// timeoutRetryDelay paces retries after a timeout fails to persist, so a flag
// that is already down does not re-fire in a tight loop.
const timeoutRetryDelay = time.Second

func (s roomState) clockRunning(g setup) bool {
	return g.Clock.Enabled && len(s.acted) == len(g.Players) && !s.finished()
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

func (s roomState) untilFlag(g setup, at time.Time) (time.Duration, bool) {
	if !s.clockRunning(g) {
		return 0, false
	}
	remaining := s.charged(g, at)
	soonest, found := time.Duration(0), false
	for _, user := range s.toAct(g) {
		if !found || remaining[user] < soonest {
			soonest, found = remaining[user], true
		}
	}
	return soonest, found
}

func (s roomState) flagged(g setup, at time.Time) (game.UserID, bool) {
	if !s.clockRunning(g) {
		return "", false
	}
	remaining := s.charged(g, at)
	for _, user := range s.toAct(g) {
		if remaining[user] <= 0 {
			return user, true
		}
	}
	return "", false
}

func (s roomState) remainingMS() map[game.UserID]int64 {
	return toMS(s.remaining)
}

func (s roomState) wireClock(g setup, at time.Time) *protocol.WireClock {
	if !g.Clock.Enabled {
		return nil
	}
	clock := &protocol.WireClock{RemainingMS: toMS(s.charged(g, at))}
	if users := s.toAct(g); s.clockRunning(g) && len(users) > 0 {
		clock.Running = &users[0]
	}
	return clock
}

func toMS(remaining map[game.UserID]time.Duration) map[game.UserID]int64 {
	if remaining == nil {
		return nil
	}
	ms := make(map[game.UserID]int64, len(remaining))
	for user, d := range remaining {
		ms[user] = d.Milliseconds()
	}
	return ms
}

func fromMS(ms map[game.UserID]int64) map[game.UserID]time.Duration {
	remaining := make(map[game.UserID]time.Duration, len(ms))
	for user, v := range ms {
		remaining[user] = time.Duration(v) * time.Millisecond
	}
	return remaining
}

func (r *Room) armTimer() {
	r.stopTimer()
	if d, ok := r.cur.untilFlag(r.setup, r.setup.Time.Now()); ok {
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
	user, ok := r.cur.flagged(r.setup, now)
	if !ok {
		return
	}
	rec := store.StepRecord{Seq: r.cur.seq + 1, User: user, Kind: store.StepTimeout, At: now}
	if _, err := r.commit(rec); err != nil {
		slog.Error("room: persist timeout", "game", r.setup.ID, "user", user, "err", err)
		r.armIn(timeoutRetryDelay)
	}
}
