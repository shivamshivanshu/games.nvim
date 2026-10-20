package auth

import (
	"net/netip"
	"testing"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/internal/clocktest"
)

var testIP = netip.MustParseAddr("203.0.113.7")

func newTestLockout() (*Lockout, *clocktest.FakeClock) {
	clock := clocktest.New(time.Unix(1_700_000_000, 0))
	return NewLockout(5, 5*time.Minute, 5*time.Minute, clock.Now), clock
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
