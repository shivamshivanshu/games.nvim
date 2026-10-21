package auth

import (
	"net/netip"
	"testing"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/internal/clocktest"
	"github.com/shivamshivanshu/games.nvim/server/internal/limits"
)

var testIP = netip.MustParseAddr("203.0.113.7")

func newTestLockout() (*Lockout, *clocktest.FakeClock) {
	clock := clocktest.New(clocktest.Epoch)
	lim := limits.Limits{LockoutFailures: 5, LockoutWindow: 5 * time.Minute, LockoutDuration: 5 * time.Minute}
	return NewLockout(lim, clock), clock
}

func recordFailures(l *Lockout, n int) {
	for range n {
		l.RecordFailure(testIP)
	}
}

func TestLockoutAfterFiveFailuresInWindow(t *testing.T) {
	l, _ := newTestLockout()
	recordFailures(l, 4)
	if l.Locked(testIP) {
		t.Fatal("locked after 4 failures")
	}
	l.RecordFailure(testIP)
	if !l.Locked(testIP) {
		t.Fatal("not locked after 5 failures")
	}
}

func TestLockoutExpires(t *testing.T) {
	l, clock := newTestLockout()
	recordFailures(l, 5)
	clock.Advance(5*time.Minute + time.Second)
	if l.Locked(testIP) {
		t.Fatal("still locked after duration elapsed")
	}
	recordFailures(l, 4)
	if l.Locked(testIP) {
		t.Fatal("failure history survived lock expiry")
	}
}

func TestFailuresOutsideWindowDontCount(t *testing.T) {
	l, clock := newTestLockout()
	recordFailures(l, 4)
	clock.Advance(6 * time.Minute)
	l.RecordFailure(testIP)
	if l.Locked(testIP) {
		t.Fatal("stale failures counted toward lockout")
	}
}

func TestStaleRecordsArePruned(t *testing.T) {
	otherIP := netip.MustParseAddr("198.51.100.1")
	tests := map[string]struct {
		failures int
		wait     time.Duration
		next     func(l *Lockout)
	}{
		"failures past the window, on the next Locked":  {1, 6 * time.Minute, func(l *Lockout) { l.Locked(otherIP) }},
		"failures past the window, on the next failure": {1, 6 * time.Minute, func(l *Lockout) { l.RecordFailure(otherIP) }},
		"an expired lock, on the next failure":          {5, 6 * time.Minute, func(l *Lockout) { l.RecordFailure(otherIP) }},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			l, clock := newTestLockout()
			recordFailures(l, tc.failures)
			clock.Advance(tc.wait)
			tc.next(l)
			if _, kept := l.records[testIP]; kept {
				t.Fatalf("stale record of %v kept: %+v", testIP, l.records[testIP])
			}
		})
	}
}
