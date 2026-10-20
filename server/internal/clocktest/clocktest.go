package clocktest

import (
	"slices"
	"sync"
	"time"

	"games.nvim/server/internal/clock"
)

type timer struct {
	clock   *FakeClock
	at      time.Time
	f       func()
	stopped bool
}

func (t *timer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	wasActive := !t.stopped
	t.stopped = true
	return wasActive
}

type FakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*timer
}

func New(start time.Time) *FakeClock { return &FakeClock{now: start} }

func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FakeClock) AfterFunc(d time.Duration, f func()) clock.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &timer{clock: c, at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

func (c *FakeClock) Armed() []time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	var deadlines []time.Time
	for _, t := range c.timers {
		if !t.stopped {
			deadlines = append(deadlines, t.at)
		}
	}
	return deadlines
}

// Advance moves time forward and runs every timer that came due, in deadline
// order, on the caller's goroutine. It returns how many fired.
func (c *FakeClock) Advance(d time.Duration) int {
	c.mu.Lock()
	c.now = c.now.Add(d)
	var due []*timer
	for _, t := range c.timers {
		if !t.stopped && !t.at.After(c.now) {
			t.stopped = true
			due = append(due, t)
		}
	}
	c.mu.Unlock()
	slices.SortFunc(due, func(a, b *timer) int { return a.at.Compare(b.at) })
	for _, t := range due {
		t.f()
	}
	return len(due)
}
