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
