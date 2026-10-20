package room

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
	"github.com/shivamshivanshu/games.nvim/server/internal/clocktest"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
	carol game.UserID = "carol"
)

const waitTimeout = 5 * time.Second

type fakePersister struct {
	mu       sync.Mutex
	steps    []store.StepRecord
	finished *store.Finish
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

func (p *fakePersister) FinishGame(_ context.Context, _ game.GameID, f store.Finish) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.attempts++
	if p.fail != nil {
		return p.fail
	}
	p.finished = &f
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

func (p *fakePersister) recorded() ([]store.StepRecord, *store.Finish) {
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

func newHarness(t *testing.T, timeControl game.TimeControl) *harness {
	t.Helper()
	return startHarness(t, game.Adapt(chess.Rules{}), `{"color":"white","time_control":"`+string(timeControl)+`"}`, bob)
}

func startHarness(t *testing.T, g game.Game, options string, invitees ...game.UserID) *harness {
	t.Helper()
	opts, err := g.ResolveOptions(json.RawMessage(options))
	if err != nil {
		t.Fatal(err)
	}
	const seed = 42
	state, seats, err := g.New(opts, alice, invitees, game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{
		persist: &fakePersister{},
		sent:    &recordingBroadcaster{},
		clock:   clocktest.New(clocktest.Epoch),
	}
	players, err := game.PlayersOf(append([]game.UserID{alice}, invitees...), seats)
	if err != nil {
		t.Fatal(err)
	}
	h.params = Params{
		ID: "g1", Game: g, Options: opts, Seed: seed, Challenger: alice, Players: players,
		Persist: h.persist, Broadcast: h.sent, Time: h.clock,
	}
	ctx, cancel := context.WithCancel(t.Context())
	h.cancel = cancel
	t.Cleanup(cancel)
	h.room = Start(ctx, newSetup(t, h.params), state)
	return h
}

func newSetup(t *testing.T, p Params) Setup {
	t.Helper()
	g, err := NewSetup(p)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func move(san string) json.RawMessage {
	return json.RawMessage(`{"kind":"move","move":"` + san + `"}`)
}

func (h *harness) play(t *testing.T, user game.UserID, seq game.Seq, san string) game.Seq {
	t.Helper()
	next, err := h.room.Act(t.Context(), user, seq, move(san))
	if err != nil {
		t.Fatalf("%s plays %s at seq %d: %v", user, san, seq, err)
	}
	return next
}

func (h *harness) control(t *testing.T, user game.UserID, seq game.Seq, c protocol.Control) game.Seq {
	t.Helper()
	next, err := h.room.Control(t.Context(), user, seq, c)
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
	s, err := h.room.Snapshot(t.Context(), user)
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

func TestNewSetupRejectsChallengerOutsidePlayers(t *testing.T) {
	h := newHarness(t, game.TimeControlNone)
	noPlayers, outsider := h.params, h.params
	noPlayers.Players = nil
	outsider.Challenger = carol
	for name, p := range map[string]Params{"no players": noPlayers, "challenger absent": outsider} {
		if _, err := NewSetup(p); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestUpdateDoesNotShareRoomState(t *testing.T) {
	h := newHarness(t, game.TimeControlNone)
	u := h.snapshot(t, alice)
	u.Players[0].User = carol
	u.Options[game.OptionTimeControl] = "1+0"
	if got := h.snapshot(t, alice); got.Players[0].User != alice || got.Options[game.OptionTimeControl] != "none" {
		t.Fatalf("mutating an update changed the room: players %v, options %v", got.Players, got.Options)
	}
}

func TestMovedEventWireShape(t *testing.T) {
	h := newHarness(t, game.TimeControlNone)
	h.play(t, alice, 0, "e4")
	events := h.snapshot(t, bob).Events
	if want := `{"by":"white","kind":"moved","san":"e4"}`; len(events) != 1 || string(events[0]) != want {
		t.Fatalf("events %s, want [%s]", events, want)
	}
}

func TestStaleSeqRejected(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	if _, err := h.room.Act(t.Context(), alice, 1, move("e4")); !errors.Is(err, ErrStale) {
		t.Fatalf("want ErrStale, got %v", err)
	}
	seq := h.play(t, alice, 0, "e4")
	if _, err := h.room.Control(t.Context(), bob, seq-1, protocol.ControlResign); !errors.Is(err, ErrStale) {
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
	h := newHarness(t, game.TimeControl10Plus5)
	if _, err := h.room.Act(t.Context(), bob, 0, move("e5")); !errors.Is(err, game.ErrNotYourTurn) {
		t.Fatalf("want ErrNotYourTurn, got %v", err)
	}
	if _, n := h.sent.last(alice); n != 0 {
		t.Fatalf("rejected action broadcast %d updates", n)
	}
}

func TestNonPlayerRejected(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	if _, err := h.room.Act(t.Context(), carol, 0, move("e4")); !errors.Is(err, ErrNotAPlayer) {
		t.Fatalf("act: want ErrNotAPlayer, got %v", err)
	}
	if _, err := h.room.Snapshot(t.Context(), carol); !errors.Is(err, ErrNotAPlayer) {
		t.Fatalf("snapshot: want ErrNotAPlayer, got %v", err)
	}
}

func TestResignEndsGame(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	seq := h.play(t, alice, 0, "e4")
	h.control(t, bob, seq, protocol.ControlResign)
	waitDone(t, h.room)

	steps, finished := h.persist.recorded()
	if finished == nil || finished.Reason != game.ReasonResign || finished.Kind != game.ResultDecisive || finished.Seq != 2 {
		t.Fatalf("finish record %+v", finished)
	}
	want := map[game.UserID]store.Standing{alice: {Outcome: store.OutcomeWin, Score: 1}, bob: {Outcome: store.OutcomeLoss, Score: -1}}
	if diff := cmp.Diff(want, finished.Standings); diff != "" {
		t.Fatalf("standings (-want +got):\n%s", diff)
	}
	if len(steps) != 1 {
		t.Fatalf("persisted %d steps, want only the move: the finish replaces steps", len(steps))
	}
	for _, user := range []game.UserID{alice, bob} {
		u, _ := h.sent.last(user)
		want := &protocol.WireResult{Kind: protocol.ResultDecisive, Winners: []game.UserID{alice}, Reason: game.ReasonResign}
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
		h := newHarness(t, game.TimeControl10Plus5)
		seq := h.control(t, alice, 0, protocol.ControlOfferDraw)
		h.control(t, bob, seq, protocol.ControlOfferDraw)
		waitDone(t, h.room)
		_, finished := h.persist.recorded()
		if finished == nil || finished.Reason != game.ReasonDrawAgreed || finished.Kind != game.ResultDraw {
			t.Fatalf("finish record %+v", finished)
		}
		draw := store.Standing{Outcome: store.OutcomeDraw}
		if diff := cmp.Diff(map[game.UserID]store.Standing{alice: draw, bob: draw}, finished.Standings); diff != "" {
			t.Fatalf("standings (-want +got):\n%s", diff)
		}
	})
	t.Run("decline", func(t *testing.T) {
		h := newHarness(t, game.TimeControl10Plus5)
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
		h := newHarness(t, game.TimeControl10Plus5)
		seq := h.play(t, alice, 0, "e4")
		seq = h.control(t, alice, seq, protocol.ControlOfferDraw)
		h.play(t, bob, seq, "e5")
		if got := h.snapshot(t, alice).DrawAgreed; got != nil {
			t.Fatalf("draw_agreed %v after a move", got)
		}
	})
}

func TestAbortWindow(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	seq := h.play(t, alice, 0, "e4")
	if !slices.Contains(h.snapshot(t, bob).Controls, protocol.ControlAbort) {
		t.Fatal("abort missing after one player's first move")
	}
	h.play(t, bob, seq, "e5")
	if controls := h.snapshot(t, alice).Controls; slices.Contains(controls, protocol.ControlAbort) {
		t.Fatalf("abort still offered after both moved: %v", controls)
	}
	if _, err := h.room.Control(t.Context(), alice, seq+1, protocol.ControlAbort); !isCode(err, protocol.CodeIllegalAction) {
		t.Fatalf("want illegal_action, got %v", err)
	}

	early := newHarness(t, game.TimeControl10Plus5)
	early.control(t, bob, early.play(t, alice, 0, "e4"), protocol.ControlAbort)
	waitDone(t, early.room)
	_, finished := early.persist.recorded()
	none := store.Standing{Outcome: store.OutcomeNone}
	if finished == nil || finished.Reason != game.ReasonAborted || finished.Kind != game.ResultAborted ||
		!maps.Equal(finished.Standings, map[game.UserID]store.Standing{alice: none, bob: none}) {
		t.Fatalf("abort finish record %+v", finished)
	}
}

func TestTimeoutEliminates(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	h.play(t, bob, h.play(t, alice, 0, "e4"), "e5")
	h.clock.Advance(10 * time.Minute)
	waitDone(t, h.room)

	_, finished := h.persist.recorded()
	if finished == nil || finished.Reason != game.ReasonTimeout || !finished.At.Equal(h.clock.Now()) {
		t.Fatalf("finish record %+v at %v", finished, h.clock.Now())
	}
	want := map[game.UserID]store.Standing{alice: {Outcome: store.OutcomeLoss, Score: -1}, bob: {Outcome: store.OutcomeWin, Score: 1}}
	if diff := cmp.Diff(want, finished.Standings); diff != "" {
		t.Fatalf("standings (-want +got):\n%s", diff)
	}
	if u, _ := h.sent.last(bob); u.Result == nil || u.Result.Reason != game.ReasonTimeout {
		t.Fatalf("bob's final update result %+v", u.Result)
	}
}

func TestClockNotChargedBeforeFirstMoves(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
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
	want := &protocol.WireClock{RemainingMS: map[game.UserID]int64{alice: 540_000, bob: 600_000}, Running: new(alice)}
	if diff := cmp.Diff(want, clock); diff != "" {
		t.Fatalf("clock once running (-want +got):\n%s", diff)
	}

	h.play(t, alice, seq, "Nf3")
	clock = h.snapshot(t, bob).Clock
	want = &protocol.WireClock{RemainingMS: map[game.UserID]int64{alice: 545_000, bob: 600_000}, Running: new(bob)}
	if diff := cmp.Diff(want, clock); diff != "" {
		t.Fatalf("clock after an incremented move (-want +got):\n%s", diff)
	}
}

func TestRestoreReplaysAndResumesClock(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	seq := game.Seq(0)
	for i, san := range []string{"e4", "e5", "Nf3", "Nc6"} {
		h.clock.Advance(time.Duration(i+1) * 7 * time.Second)
		seq = h.play(t, []game.UserID{alice, bob}[i%2], seq, san)
	}
	want := map[game.UserID]Snapshot{alice: h.fullSnapshot(t, alice), bob: h.fullSnapshot(t, bob)}
	h.cancel()

	steps, _ := h.persist.recorded()
	h.clock.Advance(time.Hour)
	restored, err := Restore(t.Context(), newSetup(t, h.params), steps)
	if err != nil {
		t.Fatal(err)
	}
	for user, original := range want {
		got, err := restored.Snapshot(t.Context(), user)
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
	h := newHarness(t, game.TimeControl10Plus5)
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
	h := newHarness(t, game.TimeControl10Plus5)
	diskFull := errors.New("disk full")
	h.persist.fail = diskFull
	if _, err := h.room.Act(t.Context(), alice, 0, move("e4")); !errors.Is(err, diskFull) {
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
	h := newHarness(t, game.TimeControl10Plus5)
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
	h := newHarness(t, game.TimeControlNone)
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

func TestResignedPlayerSpectatesWhileOthersPlayOn(t *testing.T) {
	h := newRelayHarness(t, game.TimeControlNone, bob, carol)
	seq := h.control(t, bob, h.relay(t, alice, 0, relayPass), protocol.ControlResign)
	if u, _ := h.sent.last(bob); len(u.Controls) != 0 || u.Result != nil {
		t.Fatalf("resigned player: controls %v, result %+v; want no controls in a running game", u.Controls, u.Result)
	}
	for _, c := range []protocol.Control{protocol.ControlResign, protocol.ControlAbort} {
		if _, err := h.room.Control(t.Context(), bob, seq, c); !isCode(err, protocol.CodeIllegalAction) {
			t.Errorf("resigned player's %s: want illegal_action, got %v", c, err)
		}
	}
	_, before := h.sent.last(bob)
	h.relay(t, alice, h.relay(t, carol, seq, relayPass), relayPass)
	if u, after := h.sent.last(bob); after != before+2 || !slices.Equal(u.ToAct, []game.UserID{carol}) {
		t.Fatalf("spectator got %d more updates, to_act %v; want 2 more and carol to act", after-before, u.ToAct)
	}
}

func TestClockStartsOnceRemainingPlayersActed(t *testing.T) {
	h := newRelayHarness(t, game.TimeControl10Plus5, bob, carol)
	seq := h.control(t, bob, h.relay(t, alice, 0, relayPass), protocol.ControlResign)
	h.relay(t, carol, seq, relayPass)
	h.clock.Advance(time.Minute)
	u := h.snapshot(t, alice)
	want := &protocol.WireClock{RemainingMS: map[game.UserID]int64{alice: 540_000, bob: 600_000, carol: 600_000}, Running: new(alice)}
	if diff := cmp.Diff(want, u.Clock); diff != "" {
		t.Fatalf("clock after the remaining players acted (-want +got):\n%s", diff)
	}
	if slices.Contains(u.Controls, protocol.ControlAbort) {
		t.Fatalf("abort still offered after every remaining player acted: %v", u.Controls)
	}
}

func TestIncrementOncePerTurnHandover(t *testing.T) {
	h := newRelayHarness(t, game.TimeControl10Plus5, bob)
	seq := h.relay(t, bob, h.relay(t, alice, 0, relayPass), relayPass)
	for _, m := range []relayMove{relayKeep, relayPass} {
		h.clock.Advance(10 * time.Second)
		seq = h.relay(t, alice, seq, m)
	}
	want := &protocol.WireClock{RemainingMS: map[game.UserID]int64{alice: 585_000, bob: 600_000}, Running: new(bob)}
	if diff := cmp.Diff(want, h.snapshot(t, alice).Clock); diff != "" {
		t.Fatalf("clock after a two-action turn (-want +got):\n%s", diff)
	}
}

type controlStep struct {
	user    game.UserID
	control protocol.Control
}

func (h *harness) controls(t *testing.T, seq game.Seq, steps []controlStep) {
	t.Helper()
	for _, st := range steps {
		seq = h.control(t, st.user, seq, st.control)
	}
}

func (h *harness) finished(t *testing.T) *store.Finish {
	t.Helper()
	waitDone(t, h.room)
	_, finished := h.persist.recorded()
	if finished == nil {
		t.Fatal("finished room recorded no finish")
	}
	return finished
}

func TestImposedEndingKeepsEliminatedPlayersLoss(t *testing.T) {
	loss := store.Standing{Outcome: store.OutcomeLoss, Score: -1}
	draw, none := store.Standing{Outcome: store.OutcomeDraw}, store.Standing{Outcome: store.OutcomeNone}
	tests := []struct {
		name   string
		steps  []controlStep
		reason game.ResultReason
		want   map[game.UserID]store.Standing
	}{
		{
			"draw offered after a resign",
			[]controlStep{{bob, protocol.ControlResign}, {alice, protocol.ControlOfferDraw}, {carol, protocol.ControlOfferDraw}},
			game.ReasonDrawAgreed, map[game.UserID]store.Standing{alice: draw, bob: loss, carol: draw},
		},
		{
			"holdout resigns after the others offered",
			[]controlStep{{alice, protocol.ControlOfferDraw}, {carol, protocol.ControlOfferDraw}, {bob, protocol.ControlResign}},
			game.ReasonDrawAgreed, map[game.UserID]store.Standing{alice: draw, bob: loss, carol: draw},
		},
		{
			"abort after a resign",
			[]controlStep{{bob, protocol.ControlResign}, {alice, protocol.ControlAbort}},
			game.ReasonAborted, map[game.UserID]store.Standing{alice: none, bob: loss, carol: none},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRelayHarness(t, game.TimeControlNone, bob, carol)
			h.controls(t, 0, tt.steps)
			finished := h.finished(t)
			if finished.Reason != tt.reason {
				t.Fatalf("finish reason %s, want %s", finished.Reason, tt.reason)
			}
			if diff := cmp.Diff(tt.want, finished.Standings); diff != "" {
				t.Fatalf("standings (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResignWithdrawsOwnDrawOffer(t *testing.T) {
	h := newRelayHarness(t, game.TimeControlNone, bob, carol)
	h.controls(t, 0, []controlStep{{bob, protocol.ControlOfferDraw}, {bob, protocol.ControlResign}})
	for _, user := range []game.UserID{alice, carol} {
		u := h.snapshot(t, user)
		if len(u.DrawAgreed) != 0 || slices.Contains(u.Controls, protocol.ControlDeclineDraw) {
			t.Errorf("%s: draw_agreed %v, controls %v; want no pending offer", user, u.DrawAgreed, u.Controls)
		}
	}
}

func TestEliminationEndingGameKeepsItsResultOverOpenOffers(t *testing.T) {
	h := newRelayHarness(t, game.TimeControlNone, bob)
	h.controls(t, 0, []controlStep{{alice, protocol.ControlOfferDraw}, {bob, protocol.ControlResign}})
	finished := h.finished(t)
	want := map[game.UserID]store.Standing{alice: {Outcome: store.OutcomeWin, Score: 1}, bob: {Outcome: store.OutcomeLoss, Score: -1}}
	if finished.Reason != game.ReasonResign || !maps.Equal(finished.Standings, want) {
		t.Fatalf("finish %+v, want alice winning by resign", finished)
	}
}

func TestClockStartsWhenEliminationMeetsBarrier(t *testing.T) {
	h := newRelayHarness(t, game.TimeControl10Plus5, bob, carol)
	seq := h.relay(t, bob, h.relay(t, alice, 0, relayPass), relayPass)
	h.clock.Advance(time.Minute)
	h.control(t, carol, seq, protocol.ControlResign)
	h.clock.Advance(time.Minute)
	want := &protocol.WireClock{RemainingMS: map[game.UserID]int64{alice: 540_000, bob: 600_000, carol: 600_000}, Running: new(alice)}
	if diff := cmp.Diff(want, h.snapshot(t, alice).Clock); diff != "" {
		t.Fatalf("clock a minute after the resign that met the barrier (-want +got):\n%s", diff)
	}
}

// timedOutOnOwnTurn plays a 3-player relay to the point where bob flags on
// his own turn, after alice has earned an increment.
func timedOutOnOwnTurn(t *testing.T) *harness {
	t.Helper()
	h := newRelayHarness(t, game.TimeControl10Plus5, bob, carol)
	seq := h.relay(t, carol, h.relay(t, bob, h.relay(t, alice, 0, relayPass), relayPass), relayPass)
	h.clock.Advance(10 * time.Second)
	h.relay(t, alice, seq, relayPass)
	h.clock.Advance(10 * time.Minute)
	return h
}

func TestTimeoutOnOwnTurnPassesClockToNextPlayer(t *testing.T) {
	h := timedOutOnOwnTurn(t)
	if controls := h.snapshot(t, bob).Controls; len(controls) != 0 {
		t.Fatalf("timed-out player keeps controls %v", controls)
	}
	h.clock.Advance(time.Minute)
	want := &protocol.WireClock{RemainingMS: map[game.UserID]int64{alice: 595_000, bob: 0, carol: 540_000}, Running: new(carol)}
	if diff := cmp.Diff(want, h.snapshot(t, alice).Clock); diff != "" {
		t.Fatalf("clock a minute after bob flagged (-want +got):\n%s", diff)
	}
}

func TestRestoreAfterTimeoutRearmsTimer(t *testing.T) {
	h := timedOutOnOwnTurn(t)
	want := h.fullSnapshot(t, bob)
	h.cancel()

	steps, _ := h.persist.recorded()
	restored, err := Restore(t.Context(), newSetup(t, h.params), steps)
	if err != nil {
		t.Fatal(err)
	}
	got, err := restored.Snapshot(t.Context(), bob)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("bob's snapshot after restore (-want +got):\n%s", diff)
	}
	h.clock.Advance(10 * time.Minute)
	waitDone(t, restored)
	_, finished := h.persist.recorded()
	wantStandings := map[game.UserID]store.Standing{alice: {Outcome: store.OutcomeWin, Score: 1}, bob: {Outcome: store.OutcomeLoss, Score: -1}, carol: {Outcome: store.OutcomeLoss, Score: -1}}
	if finished == nil || finished.Reason != game.ReasonTimeout || !maps.Equal(finished.Standings, wantStandings) {
		t.Fatalf("finish %+v, want carol flagging after restore to leave alice the winner", finished)
	}
}

func TestRestoreKeepsEliminatedPlayer(t *testing.T) {
	h := newRelayHarness(t, game.TimeControl10Plus5, bob, carol)
	seq := h.control(t, bob, h.relay(t, alice, 0, relayPass), protocol.ControlResign)
	seq = h.relay(t, carol, seq, relayPass)
	users := []game.UserID{alice, bob, carol}
	want := make(map[game.UserID]Snapshot, len(users))
	for _, user := range users {
		want[user] = h.fullSnapshot(t, user)
	}
	h.cancel()

	steps, _ := h.persist.recorded()
	restored, err := Restore(t.Context(), newSetup(t, h.params), steps)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		got, err := restored.Snapshot(t.Context(), user)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want[user], got); diff != "" {
			t.Errorf("%s snapshot after restore (-want +got):\n%s", user, diff)
		}
	}
	if _, err := restored.Control(t.Context(), bob, seq, protocol.ControlResign); !isCode(err, protocol.CodeIllegalAction) {
		t.Fatalf("resigned player's resign after restore: want illegal_action, got %v", err)
	}
}
