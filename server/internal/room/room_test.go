package room

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"games.nvim/server/game"
	"games.nvim/server/games/chess"
	"games.nvim/server/internal/clocktest"
	"games.nvim/server/internal/protocol"
	"games.nvim/server/internal/store"
	"games.nvim/server/internal/xslices"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
	carol game.UserID = "carol"
)

const waitTimeout = 5 * time.Second

type finishRecord struct {
	reason   game.ResultReason
	outcomes map[game.UserID]store.Outcome
}

type fakePersister struct {
	mu       sync.Mutex
	steps    []store.StepRecord
	finished *finishRecord
	fail     error
	attempts int
}

func (p *fakePersister) AppendStep(_ context.Context, _ game.GameID, step store.StepRecord) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.attempts++
	if p.fail != nil {
		return p.fail
	}
	p.steps = append(p.steps, step)
	return nil
}

func (p *fakePersister) FinishGame(_ context.Context, _ game.GameID, step store.StepRecord, reason game.ResultReason, outcomes map[game.UserID]store.Outcome) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.attempts++
	if p.fail != nil {
		return p.fail
	}
	p.steps = append(p.steps, step)
	p.finished = &finishRecord{reason: reason, outcomes: outcomes}
	return nil
}

func (p *fakePersister) setFail(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fail = err
}

func (p *fakePersister) attempted() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.attempts
}

func (p *fakePersister) recorded() ([]store.StepRecord, *finishRecord) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.steps), p.finished
}

type recordingBroadcaster struct {
	mu      sync.Mutex
	updates map[game.UserID][]protocol.GameUpdate
}

func (b *recordingBroadcaster) SendUpdate(to game.UserID, u protocol.GameUpdate) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.updates == nil {
		b.updates = map[game.UserID][]protocol.GameUpdate{}
	}
	b.updates[to] = append(b.updates[to], u)
}

func (b *recordingBroadcaster) last(user game.UserID) (protocol.GameUpdate, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	got := b.updates[user]
	if len(got) == 0 {
		return protocol.GameUpdate{}, 0
	}
	return got[len(got)-1], len(got)
}

type harness struct {
	room    *Room
	params  Params
	persist *fakePersister
	sent    *recordingBroadcaster
	clock   *clocktest.FakeClock
	cancel  context.CancelFunc
}

func newHarness(t *testing.T, timeControl chess.TimeControl) *harness {
	t.Helper()
	g := game.Adapt(chess.Rules{})
	opts, err := g.ResolveOptions(json.RawMessage(`{"color":"white","time_control":"` + string(timeControl) + `"}`))
	if err != nil {
		t.Fatal(err)
	}
	const seed = 42
	state, seats, err := g.New(opts, alice, []game.UserID{bob}, game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{
		persist: &fakePersister{},
		sent:    &recordingBroadcaster{},
		clock:   clocktest.New(time.UnixMilli(1_700_000_000_000)),
	}
	h.params = Params{
		ID: "g1", Game: g, Options: opts, Seed: seed, Challenger: alice,
		Players: []game.Player{{User: alice, Seat: seats[alice]}, {User: bob, Seat: seats[bob]}},
		Persist: h.persist, Broadcast: h.sent, Time: h.clock,
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	t.Cleanup(cancel)
	if h.room, err = New(ctx, h.params, state); err != nil {
		t.Fatal(err)
	}
	return h
}

func move(san string) json.RawMessage {
	return json.RawMessage(`{"kind":"move","move":"` + san + `"}`)
}

func (h *harness) play(t *testing.T, user game.UserID, seq game.Seq, san string) game.Seq {
	t.Helper()
	next, err := h.room.Act(context.Background(), user, seq, move(san))
	if err != nil {
		t.Fatalf("%s plays %s at seq %d: %v", user, san, seq, err)
	}
	return next
}

func (h *harness) control(t *testing.T, user game.UserID, seq game.Seq, c protocol.Control) game.Seq {
	t.Helper()
	next, err := h.room.Control(context.Background(), user, seq, c)
	if err != nil {
		t.Fatalf("%s %s at seq %d: %v", user, c, seq, err)
	}
	return next
}

func (h *harness) snapshot(t *testing.T, user game.UserID) protocol.GameUpdate {
	t.Helper()
	return h.fullSnapshot(t, user).Update
}

func (h *harness) fullSnapshot(t *testing.T, user game.UserID) Snapshot {
	t.Helper()
	s, err := h.room.Snapshot(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func isCode(err error, code protocol.ErrorCode) bool {
	e, ok := errors.AsType[protocol.Error](err)
	return ok && e.Code == code
}

func waitDone(t *testing.T, r *Room) {
	t.Helper()
	select {
	case <-r.Done():
	case <-time.After(waitTimeout):
		t.Fatal("room did not finish")
	}
}

func TestStaleSeqRejected(t *testing.T) {
	h := newHarness(t, chess.TimeControl10Plus5)
	if _, err := h.room.Act(context.Background(), alice, 1, move("e4")); !errors.Is(err, ErrStale) {
		t.Fatalf("want ErrStale, got %v", err)
	}
	seq := h.play(t, alice, 0, "e4")
	if _, err := h.room.Control(context.Background(), bob, seq-1, protocol.ControlResign); !errors.Is(err, ErrStale) {
		t.Fatalf("control: want ErrStale, got %v", err)
	}
	if steps, _ := h.persist.recorded(); len(steps) != 1 {
		t.Fatalf("rejected attempts persisted: %d steps", len(steps))
	}
	if got := h.snapshot(t, bob).Seq; got != 1 {
		t.Fatalf("seq %d, want 1", got)
	}
}

func TestNotYourTurn(t *testing.T) {
	h := newHarness(t, chess.TimeControl10Plus5)
	if _, err := h.room.Act(context.Background(), bob, 0, move("e5")); !errors.Is(err, game.ErrNotYourTurn) {
		t.Fatalf("want ErrNotYourTurn, got %v", err)
	}
	if _, n := h.sent.last(alice); n != 0 {
		t.Fatalf("rejected action broadcast %d updates", n)
	}
}

func TestNonPlayerRejected(t *testing.T) {
	h := newHarness(t, chess.TimeControl10Plus5)
	if _, err := h.room.Act(context.Background(), carol, 0, move("e4")); !errors.Is(err, ErrNotAPlayer) {
		t.Fatalf("act: want ErrNotAPlayer, got %v", err)
	}
	if _, err := h.room.Snapshot(context.Background(), carol); !errors.Is(err, ErrNotAPlayer) {
		t.Fatalf("snapshot: want ErrNotAPlayer, got %v", err)
	}
}

func TestResignEndsGame(t *testing.T) {
	h := newHarness(t, chess.TimeControl10Plus5)
	seq := h.play(t, alice, 0, "e4")
	h.control(t, bob, seq, protocol.ControlResign)
	waitDone(t, h.room)

	steps, finished := h.persist.recorded()
	if finished == nil || finished.reason != game.ReasonResign {
		t.Fatalf("finish record %+v", finished)
	}
	if diff := cmp.Diff(map[game.UserID]store.Outcome{alice: store.OutcomeWin, bob: store.OutcomeLoss}, finished.outcomes); diff != "" {
		t.Fatalf("outcomes (-want +got):\n%s", diff)
	}
	if last := steps[len(steps)-1]; last.Kind != store.StepControl || string(last.Payload) != `"resign"` || last.User != bob {
		t.Fatalf("resign step %+v", last)
	}
	for _, user := range []game.UserID{alice, bob} {
		u, _ := h.sent.last(user)
		want := &protocol.WireResult{Winners: []game.UserID{alice}, Reason: game.ReasonResign}
		if diff := cmp.Diff(want, u.Result); diff != "" {
			t.Errorf("%s final result (-want +got):\n%s", user, diff)
		}
		if len(u.Controls) != 0 || len(u.ToAct) != 0 {
			t.Errorf("%s: finished game still offers controls %v / to_act %v", user, u.Controls, u.ToAct)
		}
	}
}

func TestDrawAgreeDeclineAndClearedByMove(t *testing.T) {
	t.Run("agree", func(t *testing.T) {
		h := newHarness(t, chess.TimeControl10Plus5)
		seq := h.control(t, alice, 0, protocol.ControlOfferDraw)
		h.control(t, bob, seq, protocol.ControlOfferDraw)
		waitDone(t, h.room)
		_, finished := h.persist.recorded()
		if finished == nil || finished.reason != game.ReasonDrawAgreed {
			t.Fatalf("finish record %+v", finished)
		}
		if diff := cmp.Diff(map[game.UserID]store.Outcome{alice: store.OutcomeDraw, bob: store.OutcomeDraw}, finished.outcomes); diff != "" {
			t.Fatalf("outcomes (-want +got):\n%s", diff)
		}
	})
	t.Run("decline", func(t *testing.T) {
		h := newHarness(t, chess.TimeControl10Plus5)
		seq := h.control(t, alice, 0, protocol.ControlOfferDraw)
		if got := h.snapshot(t, bob).DrawAgreed; !slices.Equal(got, []game.UserID{alice}) {
			t.Fatalf("draw_agreed %v after offer", got)
		}
		h.control(t, bob, seq, protocol.ControlDeclineDraw)
		if got := h.snapshot(t, alice).DrawAgreed; got != nil {
			t.Fatalf("draw_agreed %v after decline", got)
		}
	})
	t.Run("cleared by move", func(t *testing.T) {
		h := newHarness(t, chess.TimeControl10Plus5)
		seq := h.play(t, alice, 0, "e4")
		seq = h.control(t, alice, seq, protocol.ControlOfferDraw)
		h.play(t, bob, seq, "e5")
		if got := h.snapshot(t, alice).DrawAgreed; got != nil {
			t.Fatalf("draw_agreed %v after a move", got)
		}
	})
}

func TestAbortWindow(t *testing.T) {
	h := newHarness(t, chess.TimeControl10Plus5)
	seq := h.play(t, alice, 0, "e4")
	if !slices.Contains(h.snapshot(t, bob).Controls, protocol.ControlAbort) {
		t.Fatal("abort missing after one player's first move")
	}
	h.play(t, bob, seq, "e5")
	if controls := h.snapshot(t, alice).Controls; slices.Contains(controls, protocol.ControlAbort) {
		t.Fatalf("abort still offered after both moved: %v", controls)
	}
	if _, err := h.room.Control(context.Background(), alice, seq+1, protocol.ControlAbort); !isCode(err, protocol.CodeIllegalAction) {
		t.Fatalf("want illegal_action, got %v", err)
	}

	early := newHarness(t, chess.TimeControl10Plus5)
	early.control(t, bob, early.play(t, alice, 0, "e4"), protocol.ControlAbort)
	waitDone(t, early.room)
	_, finished := early.persist.recorded()
	if finished == nil || finished.reason != game.ReasonAborted || len(finished.outcomes) != 0 {
		t.Fatalf("abort finish record %+v", finished)
	}
}

func TestTimeoutEliminates(t *testing.T) {
	h := newHarness(t, chess.TimeControl10Plus5)
	h.play(t, bob, h.play(t, alice, 0, "e4"), "e5")
	h.clock.Advance(10 * time.Minute)
	waitDone(t, h.room)

	steps, finished := h.persist.recorded()
	if finished == nil || finished.reason != game.ReasonTimeout {
		t.Fatalf("finish record %+v", finished)
	}
	if diff := cmp.Diff(map[game.UserID]store.Outcome{alice: store.OutcomeLoss, bob: store.OutcomeWin}, finished.outcomes); diff != "" {
		t.Fatalf("outcomes (-want +got):\n%s", diff)
	}
	last := steps[len(steps)-1]
	if last.Kind != store.StepTimeout || last.User != alice || last.Payload != nil || last.ClockRemaining[alice] != 0 {
		t.Fatalf("timeout step %+v", last)
	}
	if u, _ := h.sent.last(bob); u.Result == nil || u.Result.Reason != game.ReasonTimeout {
		t.Fatalf("bob's final update result %+v", u.Result)
	}
}

func TestClockNotChargedBeforeFirstMoves(t *testing.T) {
	h := newHarness(t, chess.TimeControl10Plus5)
	initial := map[game.UserID]int64{alice: 600_000, bob: 600_000}
	h.clock.Advance(5 * time.Minute)
	seq := h.play(t, alice, 0, "e4")
	h.clock.Advance(5 * time.Minute)
	clock := h.snapshot(t, alice).Clock
	if diff := cmp.Diff(&protocol.WireClock{RemainingMS: initial}, clock); diff != "" {
		t.Fatalf("clock before both first moves (-want +got):\n%s", diff)
	}

	seq = h.play(t, bob, seq, "e5")
	h.clock.Advance(time.Minute)
	clock = h.snapshot(t, bob).Clock
	white, black := alice, bob
	want := &protocol.WireClock{RemainingMS: map[game.UserID]int64{alice: 540_000, bob: 600_000}, Running: &white}
	if diff := cmp.Diff(want, clock); diff != "" {
		t.Fatalf("clock once running (-want +got):\n%s", diff)
	}

	h.play(t, alice, seq, "Nf3")
	clock = h.snapshot(t, bob).Clock
	want = &protocol.WireClock{RemainingMS: map[game.UserID]int64{alice: 545_000, bob: 600_000}, Running: &black}
	if diff := cmp.Diff(want, clock); diff != "" {
		t.Fatalf("clock after an incremented move (-want +got):\n%s", diff)
	}
}

func TestRestoreReplaysAndResumesClock(t *testing.T) {
	h := newHarness(t, chess.TimeControl10Plus5)
	seq := game.Seq(0)
	for i, san := range []string{"e4", "e5", "Nf3", "Nc6"} {
		h.clock.Advance(time.Duration(i+1) * 7 * time.Second)
		seq = h.play(t, []game.UserID{alice, bob}[i%2], seq, san)
	}
	want := map[game.UserID]Snapshot{alice: h.fullSnapshot(t, alice), bob: h.fullSnapshot(t, bob)}
	h.cancel()

	steps, _ := h.persist.recorded()
	h.clock.Advance(time.Hour)
	restored, err := Restore(context.Background(), h.params, steps)
	if err != nil {
		t.Fatal(err)
	}
	for user, original := range want {
		got, err := restored.Snapshot(context.Background(), user)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(original, got); diff != "" {
			t.Errorf("%s snapshot after restore (-want +got):\n%s", user, diff)
		}
		if diff := cmp.Diff(xslices.MapValues(steps[len(steps)-1].ClockRemaining, time.Duration.Milliseconds), got.Update.Clock.RemainingMS); diff != "" {
			t.Errorf("%s remaining vs last step (-want +got):\n%s", user, diff)
		}
	}
}

func TestUpdateViewIsPerRecipient(t *testing.T) {
	h := newHarness(t, chess.TimeControl10Plus5)
	h.control(t, alice, 0, protocol.ControlOfferDraw)
	offerer, _ := h.sent.last(alice)
	opponent, _ := h.sent.last(bob)
	if slices.Contains(offerer.Controls, protocol.ControlOfferDraw) {
		t.Errorf("offerer can offer again: %v", offerer.Controls)
	}
	if !slices.Contains(opponent.Controls, protocol.ControlOfferDraw) {
		t.Errorf("opponent cannot agree: %v", opponent.Controls)
	}
	for _, u := range []protocol.GameUpdate{offerer, opponent} {
		if !slices.Contains(u.Controls, protocol.ControlDeclineDraw) {
			t.Errorf("decline_draw missing while an offer is pending: %v", u.Controls)
		}
	}
}

func TestPersistFailureLeavesStateUnchanged(t *testing.T) {
	h := newHarness(t, chess.TimeControl10Plus5)
	diskFull := errors.New("disk full")
	h.persist.fail = diskFull
	if _, err := h.room.Act(context.Background(), alice, 0, move("e4")); !errors.Is(err, diskFull) {
		t.Fatalf("want persist error, got %v", err)
	}
	if _, n := h.sent.last(alice); n != 0 {
		t.Fatalf("failed step broadcast %d updates", n)
	}
	h.persist.setFail(nil)
	if got := h.play(t, alice, 0, "e4"); got != 1 {
		t.Fatalf("retry seq %d, want 1", got)
	}
}

func TestFailedTimeoutPersistRetriesAfterDelay(t *testing.T) {
	h := newHarness(t, chess.TimeControl10Plus5)
	h.play(t, bob, h.play(t, alice, 0, "e4"), "e5")
	h.persist.setFail(errors.New("disk full"))
	h.clock.Advance(10 * time.Minute)

	retryAt := h.clock.Now().Add(timeoutRetryDelay)
	deadline := time.Now().Add(waitTimeout)
	for h.persist.attempted() < 3 || !slices.Equal(h.clock.Armed(), []time.Time{retryAt}) {
		if time.Now().After(deadline) {
			t.Fatalf("after a failed timeout persist: armed %v, want only %v", h.clock.Armed(), retryAt)
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	before := h.persist.attempted()
	if fired := h.clock.Advance(0); fired != 0 {
		t.Fatalf("Advance(0) fired %d timers; armed %v", fired, h.clock.Armed())
	}
	if got := h.persist.attempted(); got != before {
		t.Fatalf("persist attempts %d -> %d without the retry delay elapsing", before, got)
	}
}

func TestWireArraysAreNeverNull(t *testing.T) {
	h := newHarness(t, chess.TimeControlNone)
	h.control(t, bob, 0, protocol.ControlAbort)
	waitDone(t, h.room)
	final, _ := h.sent.last(bob)
	for _, u := range []protocol.GameUpdate{h.snapshot(t, alice), final} {
		raw, err := json.Marshal(u)
		if err != nil {
			t.Fatal(err)
		}
		var fields struct {
			Players  json.RawMessage `json:"players"`
			ToAct    json.RawMessage `json:"to_act"`
			Controls json.RawMessage `json:"controls"`
			Events   json.RawMessage `json:"events"`
			Result   struct {
				Winners json.RawMessage `json:"winners"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		for name, v := range map[string]json.RawMessage{
			"players": fields.Players, "to_act": fields.ToAct, "controls": fields.Controls,
			"events": fields.Events, "result.winners": fields.Result.Winners,
		} {
			if string(v) == "null" || len(v) == 0 {
				t.Errorf("%s encodes as %q in %s", name, v, raw)
			}
		}
	}
}
