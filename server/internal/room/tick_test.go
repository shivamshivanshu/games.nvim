package room

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/prototest"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

func away() bool { return true }

func newMetronomeHarness(t *testing.T) *harness {
	t.Helper()
	return startHarness(t, game.Adapt(gametest.MetronomeRules{}), `{}`)
}

func metronomeAction(kind game.ActionKind) json.RawMessage {
	return json.RawMessage(`{"kind":"` + string(kind) + `"}`)
}

func (h *harness) metronome(t *testing.T, seq game.Seq, kind game.ActionKind) game.Seq {
	t.Helper()
	next, err := h.room.Act(t.Context(), prototest.Alice, seq, metronomeAction(kind))
	if err != nil {
		t.Fatalf("%s at seq %d: %v", kind, seq, err)
	}
	return next
}

// advance moves the clock by d and waits for the room to settle whatever came due.
func (h *harness) advance(t *testing.T, d time.Duration) protocol.GameUpdate {
	t.Helper()
	h.clock.Advance(d)
	return h.snapshot(t, prototest.Alice)
}

func metronomeView(t *testing.T, u protocol.GameUpdate) gametest.MetronomeView {
	t.Helper()
	return gametest.MetronomeViewOf(t, u.View)
}

func stepKinds(steps []store.StepRecord) []store.StepKind {
	return xslices.Map(steps, func(st store.StepRecord) store.StepKind { return st.Kind })
}

func TestTickFiresAsARecordedStepAndRearms(t *testing.T) {
	h := newMetronomeHarness(t)
	h.snapshot(t, prototest.Alice)
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
	sent, _ := h.sent.last(prototest.Alice)
	if len(sent.Events) != 1 || sent.Seq != 1 {
		t.Fatalf("tick broadcast seq %d events %s, want seq 1 and one beat", sent.Seq, sent.Events)
	}
	if diff := cmp.Diff([]time.Time{h.clock.Now().Add(gametest.MetronomeInterval)}, h.clock.Armed()); diff != "" {
		t.Fatalf("armed after the tick (-want +got):\n%s", diff)
	}
	if steps, _ := h.persist.recorded(); len(steps) != 0 {
		t.Fatalf("a lone tick was written at once: %v", steps)
	}
	h.stop()
	steps, _ := h.persist.recorded()
	want := []store.StepRecord{{Seq: 1, Kind: store.StepTick, At: h.clock.Now()}}
	if diff := cmp.Diff(want, steps); diff != "" {
		t.Fatalf("recorded steps once stopped (-want +got):\n%s", diff)
	}
}

func TestTicksAreWrittenInBatchesAndACrashLosesOnlyTheHeldBack(t *testing.T) {
	h := newMetronomeHarness(t)
	var crashed []store.StepRecord
	for range 50 {
		seq := h.advance(t, gametest.MetronomeInterval).Seq
		crashed, _ = h.persist.recorded()
		held := int(seq) - len(crashed)
		if time.Duration(held-1)*gametest.MetronomeInterval >= tickBatchAge {
			t.Fatalf("at seq %d, %d ticks held back past a batch", seq, held)
		}
	}
	if writes := h.persist.attempted(); writes >= len(crashed) {
		t.Fatalf("%d writes for %d ticks", writes, len(crashed))
	}

	p := h.params
	p.Persist = &fakePersister{steps: crashed}
	restored, err := Restore(t.Context(), newSetup(t, p), crashed, away)
	if err != nil {
		t.Fatal(err)
	}
	s, err := restored.Snapshot(t.Context(), prototest.Alice)
	if err != nil {
		t.Fatal(err)
	}
	want := replayedMetronome(t, p)
	if diff := cmp.Diff(want, metronomeView(t, s.Update)); diff != "" || !want.Paused || s.Update.Seq != game.Seq(len(crashed))+1 {
		t.Fatalf("restored after a crash at seq %d, %d steps written; view (-replayed +room):\n%s", s.Update.Seq, len(crashed), diff)
	}
}

func TestATickCountsTheNextFromWhenItWasDueUnlessAWholeIntervalLate(t *testing.T) {
	const interval = gametest.MetronomeInterval
	for _, tc := range []struct {
		name string
		late time.Duration
		// next is when the tick after the late one is due, from the start.
		next time.Duration
	}{
		{"a third late", interval / 3, 2 * interval},
		{"exactly an interval late", interval, 3 * interval},
		{"stalled", 2*interval + interval/2, 4*interval + interval/2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newMetronomeHarness(t)
			start := h.clock.Now()
			if u := h.advance(t, interval+tc.late); u.Seq != 1 {
				t.Fatalf("seq %d after the late tick, want 1: no burst", u.Seq)
			}
			if diff := cmp.Diff([]time.Time{start.Add(tc.next)}, h.clock.Armed()); diff != "" {
				t.Fatalf("armed after the late tick (-want +got):\n%s", diff)
			}
		})
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

func TestTickingGameInputIsNeverStale(t *testing.T) {
	h := newMetronomeHarness(t)
	h.advance(t, gametest.MetronomeInterval)
	h.advance(t, gametest.MetronomeInterval)
	seq := game.Seq(2)
	for _, kind := range []game.ActionKind{gametest.MetronomeTap, gametest.MetronomeTap, gametest.MetronomePause, gametest.MetronomeResume, gametest.MetronomeTap} {
		got := h.metronome(t, 0, kind)
		if seq++; got != seq {
			t.Fatalf("%s on seq 0 got seq %d, want %d", kind, got, seq)
		}
	}
	if _, err := h.room.Act(t.Context(), prototest.Alice, seq+1, metronomeAction(gametest.MetronomeTap)); !errors.Is(err, ErrStale) {
		t.Errorf("tap on seq %d, past the game's %d: want stale, got %v", seq+1, seq, err)
	}
}

func TestPausedTickingGameInputIsNeverStale(t *testing.T) {
	h := newMetronomeHarness(t)
	h.metronome(t, 0, gametest.MetronomePause)
	for want := game.Seq(2); want <= 3; want++ {
		if got := h.metronome(t, 0, gametest.MetronomeTap); got != want {
			t.Fatalf("tap on seq 0 while paused got seq %d, want %d", got, want)
		}
	}
	if u := h.snapshot(t, prototest.Alice); !metronomeView(t, u).Paused {
		t.Fatalf("taps resumed the game: %s", u.View)
	}
}

func TestPauseStopsTicksUntilThePlayerResumes(t *testing.T) {
	h := newMetronomeHarness(t)
	h.advance(t, gametest.MetronomeInterval)
	if err := h.room.Pause(t.Context(), away); err != nil {
		t.Fatal(err)
	}
	if armed := h.clock.Armed(); len(armed) != 0 {
		t.Fatalf("armed %v while paused", armed)
	}
	if err := h.room.Pause(t.Context(), away); err != nil {
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
	h.stop()
	steps, _ := h.persist.recorded()
	want := []store.StepKind{store.StepTick, store.StepPause, store.StepAction, store.StepTick}
	if diff := cmp.Diff(want, stepKinds(steps)); diff != "" || seq != 3 {
		t.Fatalf("resume at seq %d; step kinds (-want +got):\n%s", seq, diff)
	}
}

func TestPauseIsDroppedOnceThePlayerIsBack(t *testing.T) {
	h := newMetronomeHarness(t)
	if err := h.room.Pause(t.Context(), func() bool { return false }); err != nil {
		t.Fatal(err)
	}
	if u := h.advance(t, gametest.MetronomeInterval); u.Seq != 1 || metronomeView(t, u).Paused {
		t.Fatalf("after a pause for a player who is back: seq %d, view %s", u.Seq, u.View)
	}
	h.stop()
	if steps, _ := h.persist.recorded(); !slices.Equal(stepKinds(steps), []store.StepKind{store.StepTick}) {
		t.Fatalf("recorded %v, want only the tick", stepKinds(steps))
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
				h.snapshot(t, prototest.Alice)
			}
		}},
		{"by a resignation", func(t *testing.T, h *harness) { h.control(t, prototest.Alice, 0, protocol.ControlResign) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newMetronomeHarness(t)
			tc.finish(t, h)
			waitDone(t, h.room)
			if armed := h.clock.Armed(); len(armed) != 0 {
				t.Fatalf("armed %v after the finish", armed)
			}
			if err := h.room.Pause(t.Context(), away); err != nil {
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
	h.stop()
	steps, _ := h.persist.recorded()
	if err := h.restore(t, away); err != nil {
		t.Fatal(err)
	}
	u := h.snapshot(t, prototest.Alice)
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
	h.stop()
	want := replayedMetronome(t, h.params)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("first tick after the restore (-replayed +room):\n%s", diff)
	}
}

// stop stops h's room as the server does on shutdown, writing what it held back.
func (h *harness) stop() {
	h.cancel()
	<-h.room.Exited()
}

// restore restores h's game from the steps it persisted, as the hub does when
// the server restarts.
func (h *harness) restore(t *testing.T, away func() bool) error {
	t.Helper()
	steps, _ := h.persist.recorded()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	restored, err := Restore(ctx, newSetup(t, h.params), steps, away)
	if err != nil {
		cancel()
		return err
	}
	h.room, h.cancel = restored, cancel
	return nil
}

// waitFor waits for done without sending the room anything, so only its own
// timer can have moved it.
func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRestoreWhosePauseFailsToPersistStartsAndRetriesOnItsTimer(t *testing.T) {
	failing, healthy := newMetronomeHarness(t), newMetronomeHarness(t)
	for _, h := range []*harness{failing, healthy} {
		h.advance(t, gametest.MetronomeInterval)
		h.stop()
	}
	failing.persist.setFail(errors.New("disk full"))
	for _, h := range []*harness{failing, healthy} {
		if err := h.restore(t, away); err != nil {
			t.Fatalf("restore: %v", err)
		}
	}

	paused := metronomeView(t, healthy.snapshot(t, prototest.Alice)).Paused
	if steps, _ := healthy.persist.recorded(); !slices.Equal(stepKinds(steps), []store.StepKind{store.StepTick, store.StepPause}) || !paused {
		t.Fatalf("the healthy game recorded %v, paused %t; want its tick and a pause", stepKinds(steps), paused)
	}

	if u := failing.snapshot(t, prototest.Alice); u.Seq != 1 || metronomeView(t, u).Paused {
		t.Fatalf("pause failed to persist, yet seq %d, view %s", u.Seq, u.View)
	}
	tries := failing.persist.attempted()
	failing.snapshot(t, prototest.Alice)
	if got := failing.persist.attempted(); got != tries {
		t.Fatalf("a snapshot before the retry is due wrote %d times", got-tries)
	}
	if diff := cmp.Diff([]time.Time{failing.clock.Now().Add(retryDelay)}, failing.clock.Armed()); diff != "" {
		t.Fatalf("armed after the failed pause (-want +got):\n%s", diff)
	}

	failing.clock.Advance(retryDelay)
	waitFor(t, "the timer's retry", func() bool { return failing.persist.attempted() > tries })
	failing.snapshot(t, prototest.Alice)
	if got := failing.persist.attempted(); got != tries+1 {
		t.Fatalf("one failing retry made %d attempts", got-tries)
	}

	failing.persist.setFail(nil)
	failing.clock.Advance(retryDelay)
	waitFor(t, "the pause to persist", func() bool { steps, _ := failing.persist.recorded(); return len(steps) == 2 })
	steps, _ := failing.persist.recorded()
	want := []store.StepRecord{steps[0], {Seq: 2, Kind: store.StepPause, At: failing.clock.Now()}}
	if diff := cmp.Diff(want, steps); diff != "" {
		t.Fatalf("recorded steps after the retry (-want +got):\n%s", diff)
	}
	if u := failing.snapshot(t, prototest.Alice); !metronomeView(t, u).Paused {
		t.Fatalf("not paused once the pause persisted: %s", u.View)
	}
	if armed := failing.clock.Armed(); len(armed) != 0 {
		t.Fatalf("armed %v once paused", armed)
	}
}

func TestRestorePauseThatFailedIsDroppedOnceThePlayerIsBack(t *testing.T) {
	h := newMetronomeHarness(t)
	h.advance(t, gametest.MetronomeInterval)
	h.stop()
	h.persist.setFail(errors.New("disk full"))
	gone := true
	if err := h.restore(t, func() bool { return gone }); err != nil {
		t.Fatalf("restore: %v", err)
	}
	h.snapshot(t, prototest.Alice)
	gone = false
	h.persist.setFail(nil)
	u := h.advance(t, retryDelay)
	h.stop()
	steps, _ := h.persist.recorded()
	if !slices.Equal(stepKinds(steps), []store.StepKind{store.StepTick, store.StepTick}) || u.Seq != 2 || metronomeView(t, u).Paused {
		t.Fatalf("player back before the retry: recorded %v, seq %d, view %s; want ticking on", stepKinds(steps), u.Seq, u.View)
	}
}

// replayedMetronome is the metronome's view after the room's recorded steps,
// replayed through the game alone.
func replayedMetronome(t *testing.T, p Params) gametest.MetronomeView {
	t.Helper()
	steps, _ := p.Persist.(*fakePersister).recorded()
	state, _, err := p.Game.New(p.Options, prototest.Alice, nil, game.StepRNG(p.Seed, 0))
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
	return gametest.MetronomeViewOf(t, raw)
}

func TestFailedTickBatchRetriesAfterDelay(t *testing.T) {
	h := newMetronomeHarness(t)
	h.persist.setFail(errors.New("disk full"))
	var u protocol.GameUpdate
	for h.persist.attempted() == 0 {
		u = h.advance(t, gametest.MetronomeInterval)
	}
	if diff := cmp.Diff([]time.Time{h.clock.Now().Add(retryDelay)}, h.clock.Armed()); diff != "" {
		t.Fatalf("armed after a failed batch (-want +got):\n%s", diff)
	}
	held := u.Seq
	h.persist.setFail(nil)
	if u := h.advance(t, retryDelay); u.Seq != held+1 {
		t.Fatalf("seq %d after the retry, want %d", u.Seq, held+1)
	}
	steps, _ := h.persist.recorded()
	if len(steps) != int(held)+1 || steps[len(steps)-1].Seq != held+1 {
		t.Fatalf("the retry wrote %v, want seqs 1 to %d", steps, held+1)
	}
}

func TestFailedPausePersistHoldsTicksAndRetries(t *testing.T) {
	h := newMetronomeHarness(t)
	h.persist.setFail(errors.New("disk full"))
	if err := h.room.Pause(t.Context(), away); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]time.Time{h.clock.Now().Add(retryDelay)}, h.clock.Armed()); diff != "" {
		t.Fatalf("armed after a failed pause (-want +got):\n%s", diff)
	}
	h.persist.setFail(nil)
	u := h.advance(t, retryDelay)
	if steps, _ := h.persist.recorded(); !slices.Equal(stepKinds(steps), []store.StepKind{store.StepPause}) || !metronomeView(t, u).Paused {
		t.Fatalf("after the retry recorded %v, view %s; want the pause alone", stepKinds(steps), u.View)
	}
	if armed := h.clock.Armed(); len(armed) != 0 {
		t.Fatalf("armed %v once paused", armed)
	}
}

func TestFailedPauseIsDroppedOnceThePlayerIsBack(t *testing.T) {
	h := newMetronomeHarness(t)
	h.persist.setFail(errors.New("disk full"))
	gone := true
	if err := h.room.Pause(t.Context(), func() bool { return gone }); err != nil {
		t.Fatal(err)
	}
	h.persist.setFail(nil)
	gone = false
	if u := h.advance(t, retryDelay); u.Seq != 1 || metronomeView(t, u).Paused {
		t.Fatalf("player back before the retry: seq %d, view %s; want ticking", u.Seq, u.View)
	}
}
