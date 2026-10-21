package room

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

func newMetronomeHarness(t *testing.T) *harness {
	t.Helper()
	return startHarness(t, game.Adapt(gametest.MetronomeRules{}), `{}`)
}

func metronomeAction(kind game.ActionKind) json.RawMessage {
	return json.RawMessage(`{"kind":"` + string(kind) + `"}`)
}

func (h *harness) metronome(t *testing.T, seq game.Seq, kind game.ActionKind) game.Seq {
	t.Helper()
	next, err := h.room.Act(t.Context(), alice, seq, metronomeAction(kind))
	if err != nil {
		t.Fatalf("%s at seq %d: %v", kind, seq, err)
	}
	return next
}

// advance moves the clock by d and waits for the room to settle whatever came due.
func (h *harness) advance(t *testing.T, d time.Duration) protocol.GameUpdate {
	t.Helper()
	h.clock.Advance(d)
	return h.snapshot(t, alice)
}

func metronomeView(t *testing.T, u protocol.GameUpdate) gametest.MetronomeView {
	t.Helper()
	var view gametest.MetronomeView
	if err := json.Unmarshal(u.View, &view); err != nil {
		t.Fatal(err)
	}
	return view
}

func stepKinds(steps []store.StepRecord) []store.StepKind {
	kinds := make([]store.StepKind, len(steps))
	for i, st := range steps {
		kinds[i] = st.Kind
	}
	return kinds
}

func TestTickFiresAsARecordedStepAndRearms(t *testing.T) {
	h := newMetronomeHarness(t)
	h.snapshot(t, alice)
	if diff := cmp.Diff([]time.Time{h.clock.Now().Add(gametest.MetronomeInterval)}, h.clock.Armed()); diff != "" {
		t.Fatalf("armed at start (-want +got):\n%s", diff)
	}
	if u := h.advance(t, gametest.MetronomeInterval-time.Millisecond); u.Seq != 0 {
		t.Fatalf("seq %d before the tick is due", u.Seq)
	}
	u := h.advance(t, time.Millisecond)
	if u.Seq != 1 || metronomeView(t, u).Ticks != 1 {
		t.Fatalf("after the tick: seq %d, view %s", u.Seq, u.View)
	}
	sent, _ := h.sent.last(alice)
	if len(sent.Events) != 1 || sent.Seq != 1 {
		t.Fatalf("tick broadcast seq %d events %s, want seq 1 and one beat", sent.Seq, sent.Events)
	}
	steps, _ := h.persist.recorded()
	want := []store.StepRecord{{Seq: 1, Kind: store.StepTick, At: h.clock.Now()}}
	if diff := cmp.Diff(want, steps); diff != "" {
		t.Fatalf("recorded steps (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]time.Time{h.clock.Now().Add(gametest.MetronomeInterval)}, h.clock.Armed()); diff != "" {
		t.Fatalf("armed after the tick (-want +got):\n%s", diff)
	}
}

func TestPlayerStepKeepsTheDueTick(t *testing.T) {
	h := newMetronomeHarness(t)
	due := h.clock.Now().Add(gametest.MetronomeInterval)
	h.clock.Advance(gametest.MetronomeInterval / 2)
	h.metronome(t, 0, gametest.MetronomeTap)
	if diff := cmp.Diff([]time.Time{due}, h.clock.Armed()); diff != "" {
		t.Fatalf("armed after a tap (-want +got):\n%s", diff)
	}
	if u := h.advance(t, gametest.MetronomeInterval/2); u.Seq != 2 || metronomeView(t, u).Ticks != 1 {
		t.Fatalf("at the tick's first due time: seq %d, view %s", u.Seq, u.View)
	}
}

func TestTicksDoNotMakeASeqStaleButPlayerStepsDo(t *testing.T) {
	h := newMetronomeHarness(t)
	h.advance(t, gametest.MetronomeInterval)
	h.advance(t, gametest.MetronomeInterval)
	tapped := h.metronome(t, 0, gametest.MetronomeTap)
	if tapped != 3 {
		t.Fatalf("a tap on seq 0 after two ticks got seq %d, want 3", tapped)
	}
	h.advance(t, gametest.MetronomeInterval)
	for _, seq := range []game.Seq{tapped - 1, tapped + 2} {
		if _, err := h.room.Act(t.Context(), alice, seq, metronomeAction(gametest.MetronomeTap)); !errors.Is(err, ErrStale) {
			t.Errorf("tap at seq %d around the tap at %d and a tick: want stale, got %v", seq, tapped, err)
		}
	}
	if got := h.metronome(t, tapped, gametest.MetronomeTap); got != tapped+2 {
		t.Errorf("tap on the last tap's seq got seq %d, want %d", got, tapped+2)
	}
}

func TestPauseStopsTicksUntilThePlayerResumes(t *testing.T) {
	h := newMetronomeHarness(t)
	h.advance(t, gametest.MetronomeInterval)
	if err := h.room.Pause(t.Context()); err != nil {
		t.Fatal(err)
	}
	if armed := h.clock.Armed(); len(armed) != 0 {
		t.Fatalf("armed %v while paused", armed)
	}
	if err := h.room.Pause(t.Context()); err != nil {
		t.Fatal(err)
	}
	if u := h.advance(t, time.Hour); u.Seq != 2 || !metronomeView(t, u).Paused {
		t.Fatalf("paused an hour: seq %d, view %s", u.Seq, u.View)
	}
	seq := h.metronome(t, 1, gametest.MetronomeResume)
	if diff := cmp.Diff([]time.Time{h.clock.Now().Add(gametest.MetronomeInterval)}, h.clock.Armed()); diff != "" {
		t.Fatalf("armed after resuming (-want +got):\n%s", diff)
	}
	h.advance(t, gametest.MetronomeInterval)
	steps, _ := h.persist.recorded()
	want := []store.StepKind{store.StepTick, store.StepPause, store.StepAction, store.StepTick}
	if diff := cmp.Diff(want, stepKinds(steps)); diff != "" || seq != 3 {
		t.Fatalf("resume at seq %d; step kinds (-want +got):\n%s", seq, diff)
	}
}

func TestNoTickAfterTheGameFinishes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		finish func(t *testing.T, h *harness)
	}{
		{"by a tick", func(t *testing.T, h *harness) {
			for !h.finishedYet() {
				h.clock.Advance(gametest.MetronomeInterval)
				h.snapshot(t, alice)
			}
		}},
		{"by a resignation", func(t *testing.T, h *harness) { h.control(t, alice, 0, protocol.ControlResign) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newMetronomeHarness(t)
			tc.finish(t, h)
			waitDone(t, h.room)
			if armed := h.clock.Armed(); len(armed) != 0 {
				t.Fatalf("armed %v after the finish", armed)
			}
			if err := h.room.Pause(t.Context()); err != nil {
				t.Fatalf("pause after the finish: %v", err)
			}
		})
	}
}

func (h *harness) finishedYet() bool {
	_, finished := h.persist.recorded()
	return finished != nil
}

func TestRestoreReplaysTicksAndWaitsPaused(t *testing.T) {
	h := newMetronomeHarness(t)
	seq := game.Seq(0)
	for range 3 {
		seq = h.advance(t, gametest.MetronomeInterval).Seq
	}
	seq = h.metronome(t, seq, gametest.MetronomeTap)
	before := metronomeView(t, h.advance(t, gametest.MetronomeInterval))
	h.cancel()
	<-h.room.exited

	steps, _ := h.persist.recorded()
	restored, err := Restore(t.Context(), newSetup(t, h.params), steps)
	if err != nil {
		t.Fatal(err)
	}
	h.room = restored
	u := h.snapshot(t, alice)
	before.Paused = true
	if diff := cmp.Diff(before, metronomeView(t, u)); diff != "" || u.Seq != seq+2 {
		t.Fatalf("restored at seq %d, want %d; view (-want +got):\n%s", u.Seq, seq+2, diff)
	}
	if after, _ := h.persist.recorded(); !slices.Equal(stepKinds(after[len(steps):]), []store.StepKind{store.StepPause}) {
		t.Fatalf("restore recorded %v, want one pause", stepKinds(after[len(steps):]))
	}
	if armed := h.clock.Armed(); len(armed) != 0 {
		t.Fatalf("a restored game armed %v before its player resumed", armed)
	}

	h.metronome(t, u.Seq, gametest.MetronomeResume)
	got := metronomeView(t, h.advance(t, gametest.MetronomeInterval))
	want := replayedMetronome(t, h.params)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("first tick after the restore (-replayed +room):\n%s", diff)
	}
}

// replayedMetronome is the metronome's view after the room's recorded steps,
// replayed through the game alone.
func replayedMetronome(t *testing.T, p Params) gametest.MetronomeView {
	t.Helper()
	steps, _ := p.Persist.(*fakePersister).recorded()
	state, _, err := p.Game.New(p.Options, alice, nil, game.StepRNG(p.Seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range steps {
		switch rec.Kind {
		case store.StepTick:
			state, _, err = state.Tick(game.StepRNG(p.Seed, rec.Seq))
		case store.StepPause:
			state = state.Pause()
		default:
			state, _, err = state.Apply(gametest.MetronomeSeat, rec.Payload, game.StepRNG(p.Seed, rec.Seq))
		}
		if err != nil {
			t.Fatalf("replay seq %d: %v", rec.Seq, err)
		}
	}
	raw, err := state.View(gametest.MetronomeSeat)
	if err != nil {
		t.Fatal(err)
	}
	var view gametest.MetronomeView
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	return view
}

func TestFailedTickPersistRetriesAfterDelay(t *testing.T) {
	h := newMetronomeHarness(t)
	h.persist.setFail(errors.New("disk full"))
	h.advance(t, gametest.MetronomeInterval)
	if diff := cmp.Diff([]time.Time{h.clock.Now().Add(retryDelay)}, h.clock.Armed()); diff != "" {
		t.Fatalf("armed after a failed tick (-want +got):\n%s", diff)
	}
	h.persist.setFail(nil)
	if u := h.advance(t, retryDelay); u.Seq != 1 {
		t.Fatalf("seq %d after the retry, want 1", u.Seq)
	}
}
