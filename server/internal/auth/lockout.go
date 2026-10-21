package auth

import (
	"maps"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/internal/clock"
	"github.com/shivamshivanshu/games.nvim/server/internal/limits"
)

type ipRecord struct {
	failures    []time.Time
	lockedUntil time.Time
}

type Lockout struct {
	mu        sync.Mutex
	failures  int
	window    time.Duration
	duration  time.Duration
	clock     clock.Clock
	records   map[netip.Addr]ipRecord
	nextSweep time.Time
}

func NewLockout(lim limits.Limits, c clock.Clock) *Lockout {
	return &Lockout{
		failures: lim.LockoutFailures,
		window:   lim.LockoutWindow,
		duration: lim.LockoutDuration,
		clock:    c,
		records:  make(map[netip.Addr]ipRecord),
	}
}

func (l *Lockout) Locked(ip netip.Addr) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock.Now()
	l.pruneLocked(now)
	return now.Before(l.records[ip].lockedUntil)
}

func (l *Lockout) RecordFailure(ip netip.Addr) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock.Now()
	l.pruneLocked(now)
	record := l.records[ip]
	if now.Before(record.lockedUntil) {
		return
	}
	cutoff := now.Add(-l.window)
	recent := append(slices.DeleteFunc(record.failures, func(at time.Time) bool { return at.Before(cutoff) }), now)
	if len(recent) >= l.failures {
		l.records[ip] = ipRecord{lockedUntil: now.Add(l.duration)}
		return
	}
	l.records[ip] = ipRecord{failures: recent}
}

// pruneLocked drops every record that no longer counts, so an address that
// fails once and never returns is not kept forever. It sweeps at most once a
// window, so a login does not cost a pass over every address that failed.
func (l *Lockout) pruneLocked(now time.Time) {
	if now.Before(l.nextSweep) {
		return
	}
	l.nextSweep = now.Add(l.window)
	cutoff := now.Add(-l.window)
	maps.DeleteFunc(l.records, func(_ netip.Addr, r ipRecord) bool { return r.stale(now, cutoff) })
}

// stale is whether r's lock has expired or, unlocked, its newest failure is
// older than cutoff.
func (r ipRecord) stale(now, cutoff time.Time) bool {
	if !r.lockedUntil.IsZero() {
		return !now.Before(r.lockedUntil)
	}
	return r.failures[len(r.failures)-1].Before(cutoff)
}
