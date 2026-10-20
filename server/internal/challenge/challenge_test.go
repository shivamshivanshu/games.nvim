package challenge

import (
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"games.nvim/server/game"
	"games.nvim/server/internal/clocktest"
	"games.nvim/server/internal/protocol"
	"games.nvim/server/internal/xslices"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
	carol game.UserID = "carol"

	ttl = 5 * time.Minute
)

var info = game.Info{Type: "chess", Players: game.PlayerRange{Min: 2, Max: 4}}

type harness struct {
	reg     *Registry
	clock   *clocktest.FakeClock
	mu      sync.Mutex
	changes []protocol.Challenge
	starts  atomic.Int32
	startFn StartFunc
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		clock:   clocktest.New(time.UnixMilli(1_700_000_000_000)),
		startFn: func(protocol.Challenge) (game.GameID, error) { return "g1", nil },
	}
	h.reg = NewRegistry(ttl, h.clock, func(c protocol.Challenge) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.changes = append(h.changes, c)
	}, func(c protocol.Challenge) (game.GameID, error) {
		h.starts.Add(1)
		return h.startFn(c)
	})
	return h
}

func (h *harness) states() []protocol.ChallengeState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return xslices.Map(h.changes, func(c protocol.Challenge) protocol.ChallengeState { return c.State })
}

func (h *harness) create(t *testing.T, to ...game.UserID) protocol.Challenge {
	t.Helper()
	c, err := h.reg.Create(alice, to, info, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func wantBadRequest(t *testing.T, err error) {
	t.Helper()
	if e, ok := errors.AsType[protocol.Error](err); !ok || e.Code != protocol.CodeBadRequest {
		t.Fatalf("got %v, want bad_request", err)
	}
}

func TestCreateValidation(t *testing.T) {
	cases := map[string]struct {
		to []game.UserID
	}{
		"self":       {[]game.UserID{alice}},
		"duplicate":  {[]game.UserID{bob, bob}},
		"too many":   {[]game.UserID{bob, carol, "d", "e"}},
		"empty":      {nil},
		"self among": {[]game.UserID{bob, alice}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			_, err := h.reg.Create(alice, tc.to, info, nil)
			wantBadRequest(t, err)
			if len(h.states()) != 0 {
				t.Fatal("rejected create must not notify")
			}
		})
	}
}

func TestAllAcceptStartsGame(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, bob, carol)
	for _, u := range []game.UserID{bob, bob, carol} {
		if err := h.reg.Accept(c.ChallengeID, u); err != nil {
			t.Fatal(err)
		}
	}
	want := []protocol.ChallengeState{protocol.ChallengePending, protocol.ChallengePending, protocol.ChallengeAccepted}
	if diff := cmp.Diff(want, h.states()); diff != "" {
		t.Fatal(diff)
	}
	last := h.changes[2]
	if last.GameID == nil || *last.GameID != "g1" {
		t.Fatalf("game id = %v", last.GameID)
	}
	if diff := cmp.Diff([]game.UserID{bob}, h.changes[1].Accepted); diff != "" {
		t.Fatal(diff)
	}
	if h.starts.Load() != 1 || len(h.reg.Pending(alice)) != 0 {
		t.Fatalf("starts=%d pending=%v", h.starts.Load(), h.reg.Pending(alice))
	}
	wantBadRequest(t, h.reg.Accept(c.ChallengeID, bob))
}

func TestStartFailureCancels(t *testing.T) {
	h := newHarness(t)
	h.startFn = func(protocol.Challenge) (game.GameID, error) { return "", errors.New("boom") }
	c := h.create(t, bob)
	if err := h.reg.Accept(c.ChallengeID, bob); err != nil {
		t.Fatal(err)
	}
	want := []protocol.ChallengeState{protocol.ChallengePending, protocol.ChallengeCancelled}
	if diff := cmp.Diff(want, h.states()); diff != "" {
		t.Fatal(diff)
	}
}

func TestDeclineResolves(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, bob, carol)
	wantBadRequest(t, h.reg.Decline(c.ChallengeID, "mallory"))
	if err := h.reg.Decline(c.ChallengeID, carol); err != nil {
		t.Fatal(err)
	}
	last := h.changes[len(h.changes)-1]
	if last.State != protocol.ChallengeDeclined || last.By == nil || *last.By != carol {
		t.Fatalf("got %+v", last)
	}
	wantBadRequest(t, h.reg.Accept(c.ChallengeID, bob))
}

func TestCancelOnlyByChallenger(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, bob)
	wantBadRequest(t, h.reg.Cancel(c.ChallengeID, bob))
	if err := h.reg.Cancel(c.ChallengeID, alice); err != nil {
		t.Fatal(err)
	}
	if got := h.states(); got[len(got)-1] != protocol.ChallengeCancelled {
		t.Fatalf("states %v", got)
	}
	wantBadRequest(t, h.reg.Cancel(c.ChallengeID, alice))
}

func TestExpiresAfterTTL(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, bob)
	h.clock.Advance(ttl - time.Second)
	if len(h.reg.Pending(bob)) != 1 {
		t.Fatal("expired early")
	}
	h.clock.Advance(time.Second)
	if got := h.states(); got[len(got)-1] != protocol.ChallengeExpired {
		t.Fatalf("states %v", got)
	}
	wantBadRequest(t, h.reg.Accept(c.ChallengeID, bob))
}

func TestResolutionStopsExpiry(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, bob)
	if err := h.reg.Decline(c.ChallengeID, bob); err != nil {
		t.Fatal(err)
	}
	if armed := h.clock.Armed(); len(armed) != 0 {
		t.Fatalf("timer left armed: %v", armed)
	}
	h.reg.expire(c.ChallengeID)
	if got := h.states(); len(got) != 2 {
		t.Fatalf("late expiry notified: %v", got)
	}
}

func TestPendingOrderAndScope(t *testing.T) {
	h := newHarness(t)
	if got := h.reg.Pending(alice); got == nil || len(got) != 0 {
		t.Fatalf("want empty non-nil, got %#v", got)
	}
	first := h.create(t, bob)
	h.clock.Advance(time.Second)
	second := h.create(t, carol)
	third := h.create(t, carol)
	ids := func(cs []protocol.Challenge) []protocol.ChallengeID {
		return xslices.Map(cs, func(c protocol.Challenge) protocol.ChallengeID { return c.ChallengeID })
	}
	if got := ids(h.reg.Pending(bob)); !slices.Equal(got, []protocol.ChallengeID{first.ChallengeID}) {
		t.Fatalf("bob sees %v", got)
	}
	got := h.reg.Pending(alice)
	wantTail := []protocol.ChallengeID{second.ChallengeID, third.ChallengeID}
	slices.Sort(wantTail)
	if want := append([]protocol.ChallengeID{first.ChallengeID}, wantTail...); !slices.Equal(ids(got), want) {
		t.Fatalf("alice order %v, want %v", ids(got), want)
	}
}

func TestConcurrentAcceptsStartOnce(t *testing.T) {
	h := newHarness(t)
	invitees := []game.UserID{"b", "c", "d"}
	c := h.create(t, invitees...)
	var wg sync.WaitGroup
	for _, u := range invitees {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = h.reg.Accept(c.ChallengeID, u)
		}()
	}
	wg.Wait()
	if h.starts.Load() != 1 {
		t.Fatalf("start ran %d times", h.starts.Load())
	}
}
