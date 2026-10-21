package challenge

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/clocktest"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/prototest"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

const (
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
		clock:   clocktest.New(clocktest.Epoch),
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
	c, err := h.reg.Create(prototest.Alice, to, info, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCreateValidation(t *testing.T) {
	cases := map[string]struct {
		to []game.UserID
	}{
		"self":       {[]game.UserID{prototest.Alice}},
		"duplicate":  {[]game.UserID{prototest.Bob, prototest.Bob}},
		"too many":   {[]game.UserID{prototest.Bob, prototest.Carol, "d", "e"}},
		"empty":      {nil},
		"self among": {[]game.UserID{prototest.Bob, prototest.Alice}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			_, err := h.reg.Create(prototest.Alice, tc.to, info, nil)
			prototest.WantCode(t, err, protocol.CodeBadRequest)
			if len(h.states()) != 0 {
				t.Fatal("rejected create must not notify")
			}
		})
	}
}

func TestAllAcceptStartsGame(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, prototest.Bob, prototest.Carol)
	for _, u := range []game.UserID{prototest.Bob, prototest.Bob, prototest.Carol} {
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
	if diff := cmp.Diff([]game.UserID{prototest.Bob}, h.changes[1].Accepted); diff != "" {
		t.Fatal(diff)
	}
	if h.starts.Load() != 1 || len(h.reg.Pending(prototest.Alice)) != 0 {
		t.Fatalf("starts=%d pending=%v", h.starts.Load(), h.reg.Pending(prototest.Alice))
	}
	prototest.WantCode(t, h.reg.Accept(c.ChallengeID, prototest.Bob), protocol.CodeBadRequest)
}

func TestStartFailureCancels(t *testing.T) {
	h := newHarness(t)
	h.startFn = func(protocol.Challenge) (game.GameID, error) { return "", errors.New("boom") }
	c := h.create(t, prototest.Bob)
	if err := h.reg.Accept(c.ChallengeID, prototest.Bob); err != nil {
		t.Fatal(err)
	}
	want := []protocol.ChallengeState{protocol.ChallengePending, protocol.ChallengeCancelled}
	if diff := cmp.Diff(want, h.states()); diff != "" {
		t.Fatal(diff)
	}
}

func TestDeclineResolves(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, prototest.Bob, prototest.Carol)
	prototest.WantCode(t, h.reg.Decline(c.ChallengeID, "mallory"), protocol.CodeBadRequest)
	if err := h.reg.Decline(c.ChallengeID, prototest.Carol); err != nil {
		t.Fatal(err)
	}
	last := h.changes[len(h.changes)-1]
	if last.State != protocol.ChallengeDeclined || last.By == nil || *last.By != prototest.Carol {
		t.Fatalf("got %+v", last)
	}
	prototest.WantCode(t, h.reg.Accept(c.ChallengeID, prototest.Bob), protocol.CodeBadRequest)
}

func TestCancelOnlyByChallenger(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, prototest.Bob)
	prototest.WantCode(t, h.reg.Cancel(c.ChallengeID, prototest.Bob), protocol.CodeBadRequest)
	if err := h.reg.Cancel(c.ChallengeID, prototest.Alice); err != nil {
		t.Fatal(err)
	}
	if got := h.states(); got[len(got)-1] != protocol.ChallengeCancelled {
		t.Fatalf("states %v", got)
	}
	prototest.WantCode(t, h.reg.Cancel(c.ChallengeID, prototest.Alice), protocol.CodeBadRequest)
}

func TestExpiresAfterTTL(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, prototest.Bob)
	h.clock.Advance(ttl - time.Second)
	if len(h.reg.Pending(prototest.Bob)) != 1 {
		t.Fatal("expired early")
	}
	h.clock.Advance(time.Second)
	if got := h.states(); got[len(got)-1] != protocol.ChallengeExpired {
		t.Fatalf("states %v", got)
	}
	prototest.WantCode(t, h.reg.Accept(c.ChallengeID, prototest.Bob), protocol.CodeBadRequest)
}

func TestResolutionStopsExpiry(t *testing.T) {
	h := newHarness(t)
	c := h.create(t, prototest.Bob)
	if err := h.reg.Decline(c.ChallengeID, prototest.Bob); err != nil {
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
	if got := h.reg.Pending(prototest.Alice); got == nil || len(got) != 0 {
		t.Fatalf("want empty non-nil, got %#v", got)
	}
	first := h.create(t, prototest.Bob)
	h.clock.Advance(time.Second)
	second := h.create(t, prototest.Carol)
	third := h.create(t, prototest.Carol)
	ids := func(cs []protocol.Challenge) []protocol.ChallengeID {
		return xslices.Map(cs, func(c protocol.Challenge) protocol.ChallengeID { return c.ChallengeID })
	}
	if got := ids(h.reg.Pending(prototest.Bob)); !slices.Equal(got, []protocol.ChallengeID{first.ChallengeID}) {
		t.Fatalf("bob sees %v", got)
	}
	got := h.reg.Pending(prototest.Alice)
	wantTail := []protocol.ChallengeID{second.ChallengeID, third.ChallengeID}
	slices.Sort(wantTail)
	if want := append([]protocol.ChallengeID{first.ChallengeID}, wantTail...); !slices.Equal(ids(got), want) {
		t.Fatalf("alice order %v, want %v", ids(got), want)
	}
}

func TestThreeInviteesEachSeeTheChallengeAndItStartsAfterTheLastAccept(t *testing.T) {
	h := newHarness(t)
	invitees := []game.UserID{prototest.Bob, prototest.Carol, "dave"}
	c := h.create(t, invitees...)
	for _, u := range invitees {
		if pending := h.reg.Pending(u); len(pending) != 1 || pending[0].ChallengeID != c.ChallengeID {
			t.Fatalf("%s pending %v, want only %s", u, pending, c.ChallengeID)
		}
	}
	for i, u := range invitees {
		if err := h.reg.Accept(c.ChallengeID, u); err != nil {
			t.Fatal(err)
		}
		want := int32(0)
		if i == len(invitees)-1 {
			want = 1
		}
		if got := h.starts.Load(); got != want {
			t.Fatalf("after %d of %d accepts: started %d times, want %d", i+1, len(invitees), got, want)
		}
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

func TestExpiresAtIsUnixMillisecondsOfNowPlusTheTTL(t *testing.T) {
	h := newHarness(t)
	raw, err := json.Marshal(h.create(t, prototest.Bob))
	if err != nil {
		t.Fatal(err)
	}
	want := `"expires_at":` + strconv.FormatInt(clocktest.Epoch.Add(ttl).UnixMilli(), 10)
	if !strings.Contains(string(raw), want) {
		t.Errorf("challenge %s, want %s", raw, want)
	}
}
