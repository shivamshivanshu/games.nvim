package auth

import (
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
	mu       sync.Mutex
	failures int
	window   time.Duration
	duration time.Duration
	clock    clock.Clock
	records  map[netip.Addr]ipRecord
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
	return l.lockedLocked(ip, l.clock.Now())
}

func (l *Lockout) RecordFailure(ip netip.Addr) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock.Now()
	if l.lockedLocked(ip, now) {
		return
	}
	cutoff := now.Add(-l.window)
	record := l.records[ip]
	recent := slices.DeleteFunc(slices.Clone(record.failures), func(at time.Time) bool { return at.Before(cutoff) })
	recent = append(recent, now)
	if len(recent) >= l.failures {
		l.records[ip] = ipRecord{lockedUntil: now.Add(l.duration)}
		return
	}
	l.records[ip] = ipRecord{failures: recent}
}

func (l *Lockout) lockedLocked(ip netip.Addr, now time.Time) bool {
	record, ok := l.records[ip]
	if !ok || record.lockedUntil.IsZero() {
		return false
	}
	if now.Before(record.lockedUntil) {
		return true
	}
	delete(l.records, ip)
	return false
}
