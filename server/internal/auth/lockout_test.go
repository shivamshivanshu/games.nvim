package auth

import (
	"net/netip"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

var testIP = netip.MustParseAddr("203.0.113.7")

func newTestLockout() (*Lockout, *fakeClock) {
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	return NewLockout(5, 5*time.Minute, 5*time.Minute, clock.now), clock
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
	clock.advance(5*time.Minute + time.Second)
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
	clock.advance(6 * time.Minute)
	l.RecordFailure(testIP)
	if l.Locked(testIP) {
		t.Fatal("stale failures counted toward lockout")
	}
}
