package room

import (
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/prototest"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

func TestNewSetupRejectsChallengerOutsidePlayers(t *testing.T) {
	h := newHarness(t, game.TimeControlNone)
	noPlayers, outsider := h.params, h.params
	noPlayers.Players = nil
	outsider.Challenger = prototest.Carol
	for name, p := range map[string]Params{"no players": noPlayers, "challenger absent": outsider} {
		if _, err := NewSetup(p); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestUpdateDoesNotShareRoomState(t *testing.T) {
	h := newHarness(t, game.TimeControlNone)
	u := h.snapshot(t, prototest.Alice)
	u.Players[0].User = prototest.Carol
	u.Options[game.OptionTimeControl] = "1+0"
	if got := h.snapshot(t, prototest.Alice); got.Players[0].User != prototest.Alice || got.Options[game.OptionTimeControl] != "none" {
		t.Fatalf("mutating an update changed the room: players %v, options %v", got.Players, got.Options)
	}
}

func TestMovedEventWireShape(t *testing.T) {
	h := newHarness(t, game.TimeControlNone)
	h.play(t, prototest.Alice, 0, "e4")
	events := h.snapshot(t, prototest.Bob).Events
	if want := `{"by":"white","kind":"moved","san":"e4"}`; len(events) != 1 || string(events[0]) != want {
		t.Fatalf("events %s, want [%s]", events, want)
	}
}

func TestStaleSeqRejected(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	if _, err := h.room.Act(t.Context(), prototest.Alice, 1, move("e4")); !errors.Is(err, ErrStale) {
		t.Fatalf("want ErrStale, got %v", err)
	}
	seq := h.play(t, prototest.Alice, 0, "e4")
	if _, err := h.room.Control(t.Context(), prototest.Bob, seq-1, protocol.ControlResign); !errors.Is(err, ErrStale) {
		t.Fatalf("control: want ErrStale, got %v", err)
	}
	if steps, _ := h.persist.recorded(); len(steps) != 1 {
		t.Fatalf("rejected attempts persisted: %d steps", len(steps))
	}
	if got := h.snapshot(t, prototest.Bob).Seq; got != 1 {
		t.Fatalf("seq %d, want 1", got)
	}
}

func TestNotYourTurn(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	if _, err := h.room.Act(t.Context(), prototest.Bob, 0, move("e5")); !errors.Is(err, game.ErrNotYourTurn) {
		t.Fatalf("want ErrNotYourTurn, got %v", err)
	}
	if _, n := h.sent.last(prototest.Alice); n != 0 {
		t.Fatalf("rejected action broadcast %d updates", n)
	}
}

func TestNonPlayerRejected(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	if _, err := h.room.Act(t.Context(), prototest.Carol, 0, move("e4")); !errors.Is(err, ErrNotAPlayer) {
		t.Fatalf("act: want ErrNotAPlayer, got %v", err)
	}
	if _, err := h.room.Snapshot(t.Context(), prototest.Carol); !errors.Is(err, ErrNotAPlayer) {
		t.Fatalf("snapshot: want ErrNotAPlayer, got %v", err)
	}
}

func TestResignEndsGame(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	seq := h.play(t, prototest.Alice, 0, "e4")
	h.control(t, prototest.Bob, seq, protocol.ControlResign)
	waitDone(t, h.room)

	steps, finished := h.persist.recorded()
	if finished == nil || finished.Reason != game.ReasonResign || finished.Kind != game.ResultDecisive || finished.Seq != 2 {
		t.Fatalf("finish record %+v", finished)
	}
	want := map[game.UserID]store.Standing{prototest.Alice: {Outcome: store.OutcomeWin, Score: 1}, prototest.Bob: {Outcome: store.OutcomeLoss, Score: -1}}
	if diff := cmp.Diff(want, finished.Standings); diff != "" {
		t.Fatalf("standings (-want +got):\n%s", diff)
	}
	if len(steps) != 1 {
		t.Fatalf("persisted %d steps, want only the move: the finish replaces steps", len(steps))
	}
	for _, user := range []game.UserID{prototest.Alice, prototest.Bob} {
		u, _ := h.sent.last(user)
		want := &protocol.WireResult{Kind: game.ResultDecisive, Winners: []game.UserID{prototest.Alice}, Reason: game.ReasonResign}
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
		seq := h.control(t, prototest.Alice, 0, protocol.ControlOfferDraw)
		h.control(t, prototest.Bob, seq, protocol.ControlOfferDraw)
		waitDone(t, h.room)
		_, finished := h.persist.recorded()
		if finished == nil || finished.Reason != game.ReasonDrawAgreed || finished.Kind != game.ResultDraw {
			t.Fatalf("finish record %+v", finished)
		}
		draw := store.Standing{Outcome: store.OutcomeDraw}
		if diff := cmp.Diff(map[game.UserID]store.Standing{prototest.Alice: draw, prototest.Bob: draw}, finished.Standings); diff != "" {
			t.Fatalf("standings (-want +got):\n%s", diff)
		}
	})
	t.Run("decline", func(t *testing.T) {
		h := newHarness(t, game.TimeControl10Plus5)
		seq := h.control(t, prototest.Alice, 0, protocol.ControlOfferDraw)
		if got := h.snapshot(t, prototest.Bob).DrawAgreed; !slices.Equal(got, []game.UserID{prototest.Alice}) {
			t.Fatalf("draw_agreed %v after offer", got)
		}
		h.control(t, prototest.Bob, seq, protocol.ControlDeclineDraw)
		if got := h.snapshot(t, prototest.Alice).DrawAgreed; got == nil || len(got) > 0 {
			t.Fatalf("draw_agreed %#v after decline, want empty and not null", got)
		}
	})
	t.Run("cleared by move", func(t *testing.T) {
		h := newHarness(t, game.TimeControl10Plus5)
		seq := h.play(t, prototest.Alice, 0, "e4")
		seq = h.control(t, prototest.Alice, seq, protocol.ControlOfferDraw)
		h.play(t, prototest.Bob, seq, "e5")
		if got := h.snapshot(t, prototest.Alice).DrawAgreed; got == nil || len(got) > 0 {
			t.Fatalf("draw_agreed %#v after a move, want empty and not null", got)
		}
	})
}

func TestAbortWindow(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	seq := h.play(t, prototest.Alice, 0, "e4")
	if !slices.Contains(h.snapshot(t, prototest.Bob).Controls, protocol.ControlAbort) {
		t.Fatal("abort missing after one player's first move")
	}
	h.play(t, prototest.Bob, seq, "e5")
	if controls := h.snapshot(t, prototest.Alice).Controls; slices.Contains(controls, protocol.ControlAbort) {
		t.Fatalf("abort still offered after both moved: %v", controls)
	}
	_, err := h.room.Control(t.Context(), prototest.Alice, seq+1, protocol.ControlAbort)
	prototest.WantCode(t, err, protocol.CodeIllegalAction)

	early := newHarness(t, game.TimeControl10Plus5)
	early.control(t, prototest.Bob, early.play(t, prototest.Alice, 0, "e4"), protocol.ControlAbort)
	waitDone(t, early.room)
	_, finished := early.persist.recorded()
	none := store.Standing{Outcome: store.OutcomeNone}
	if finished == nil || finished.Reason != game.ReasonAborted || finished.Kind != game.ResultAborted ||
		!maps.Equal(finished.Standings, map[game.UserID]store.Standing{prototest.Alice: none, prototest.Bob: none}) {
		t.Fatalf("abort finish record %+v", finished)
	}
}

func TestTimeoutEliminates(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	h.play(t, prototest.Bob, h.play(t, prototest.Alice, 0, "e4"), "e5")
	h.clock.Advance(10 * time.Minute)
	waitDone(t, h.room)

	_, finished := h.persist.recorded()
	if finished == nil || finished.Reason != game.ReasonTimeout || !finished.At.Equal(h.clock.Now()) {
		t.Fatalf("finish record %+v at %v", finished, h.clock.Now())
	}
	want := map[game.UserID]store.Standing{prototest.Alice: {Outcome: store.OutcomeLoss, Score: -1}, prototest.Bob: {Outcome: store.OutcomeWin, Score: 1}}
	if diff := cmp.Diff(want, finished.Standings); diff != "" {
		t.Fatalf("standings (-want +got):\n%s", diff)
	}
	if u, _ := h.sent.last(prototest.Bob); u.Result == nil || u.Result.Reason != game.ReasonTimeout {
		t.Fatalf("bob's final update result %+v", u.Result)
	}
}

func TestClockNotChargedBeforeFirstMoves(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	initial := map[game.UserID]int64{prototest.Alice: 600_000, prototest.Bob: 600_000}
	h.clock.Advance(5 * time.Minute)
	seq := h.play(t, prototest.Alice, 0, "e4")
	h.clock.Advance(5 * time.Minute)
	clock := h.snapshot(t, prototest.Alice).Clock
	if diff := cmp.Diff(&protocol.WireClock{RemainingMS: initial, AsOfMS: protocol.UnixMSOf(h.clock.Now())}, clock); diff != "" {
		t.Fatalf("clock before both first moves (-want +got):\n%s", diff)
	}

	seq = h.play(t, prototest.Bob, seq, "e5")
	h.clock.Advance(time.Minute)
	clock = h.snapshot(t, prototest.Bob).Clock
	want := &protocol.WireClock{RemainingMS: map[game.UserID]int64{prototest.Alice: 540_000, prototest.Bob: 600_000}, Running: new(prototest.Alice), AsOfMS: protocol.UnixMSOf(h.clock.Now())}
	if diff := cmp.Diff(want, clock); diff != "" {
		t.Fatalf("clock once running (-want +got):\n%s", diff)
	}

	h.play(t, prototest.Alice, seq, "Nf3")
	clock = h.snapshot(t, prototest.Bob).Clock
	want = &protocol.WireClock{RemainingMS: map[game.UserID]int64{prototest.Alice: 545_000, prototest.Bob: 600_000}, Running: new(prototest.Bob), AsOfMS: protocol.UnixMSOf(h.clock.Now())}
	if diff := cmp.Diff(want, clock); diff != "" {
		t.Fatalf("clock after an incremented move (-want +got):\n%s", diff)
	}
}

func TestRestoreReplaysAndResumesClock(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	seq := game.Seq(0)
	for i, san := range []string{"e4", "e5", "Nf3", "Nc6"} {
		h.clock.Advance(time.Duration(i+1) * 7 * time.Second)
		seq = h.play(t, []game.UserID{prototest.Alice, prototest.Bob}[i%2], seq, san)
	}
	want := map[game.UserID]Snapshot{prototest.Alice: h.fullSnapshot(t, prototest.Alice), prototest.Bob: h.fullSnapshot(t, prototest.Bob)}
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
		if diff := cmp.Diff(original, got, cmpopts.IgnoreFields(protocol.WireClock{}, "AsOfMS")); diff != "" {
			t.Errorf("%s snapshot after restore (-want +got):\n%s", user, diff)
		}
		if got.Update.Clock.AsOfMS != protocol.UnixMSOf(h.clock.Now()) {
			t.Errorf("%s restored clock as of %d, want now", user, got.Update.Clock.AsOfMS)
		}
		if diff := cmp.Diff(xslices.MapValues(steps[len(steps)-1].ClockRemaining, time.Duration.Milliseconds), got.Update.Clock.RemainingMS); diff != "" {
			t.Errorf("%s remaining vs last step (-want +got):\n%s", user, diff)
		}
	}
}

func TestUpdateViewIsPerRecipient(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	h.control(t, prototest.Alice, 0, protocol.ControlOfferDraw)
	offerer, _ := h.sent.last(prototest.Alice)
	opponent, _ := h.sent.last(prototest.Bob)
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

func TestBroadcastMatchesEachPlayersSnapshot(t *testing.T) {
	h := newRelayHarness(t, game.TimeControl10Plus5, prototest.Bob, prototest.Carol)
	h.control(t, prototest.Bob, h.relay(t, prototest.Alice, 0, relayPass), protocol.ControlOfferDraw)
	for _, user := range []game.UserID{prototest.Alice, prototest.Bob, prototest.Carol} {
		sent, _ := h.sent.last(user)
		if diff := cmp.Diff(h.snapshot(t, user), sent); diff != "" {
			t.Errorf("%s broadcast vs snapshot (-snapshot +broadcast):\n%s", user, diff)
		}
		var view relayView
		if err := json.Unmarshal(sent.View, &view); err != nil || view.Viewer != game.Seat(user) {
			t.Errorf("%s got the view of %q, %v", user, view.Viewer, err)
		}
	}
}

func TestFinishingBroadcastAddsStepToSealedUpdate(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	seq := game.Seq(0)
	for i, san := range []string{"f3", "e5", "g4"} {
		seq = h.play(t, []game.UserID{prototest.Alice, prototest.Bob}[i%2], seq, san)
	}
	h.clock.Advance(time.Minute)
	h.play(t, prototest.Bob, seq, "Qh4#")
	waitDone(t, h.room)
	wantClock := &protocol.WireClock{RemainingMS: map[game.UserID]int64{prototest.Alice: 605_000, prototest.Bob: 545_000}, AsOfMS: protocol.UnixMSOf(h.clock.Now())}
	for _, user := range []game.UserID{prototest.Alice, prototest.Bob} {
		sent, _ := h.sent.last(user)
		sealed := h.snapshot(t, user)
		if diff := cmp.Diff(wantClock, sent.Clock); diff != "" {
			t.Errorf("%s finishing clock (-want +got):\n%s", user, diff)
		}
		if want := `{"by":"black","kind":"moved","san":"Qh4#"}`; len(sent.Events) != 1 || string(sent.Events[0]) != want {
			t.Errorf("%s finishing events %s, want [%s]", user, sent.Events, want)
		}
		if len(sent.View) == 0 {
			t.Errorf("%s finishing broadcast has no view", user)
		}
		sealed.Events, sealed.Clock = sent.Events, sent.Clock
		if diff := cmp.Diff(sealed, sent); diff != "" {
			t.Errorf("%s finishing broadcast vs sealed snapshot (-sealed +broadcast):\n%s", user, diff)
		}
	}
}

func TestPersistFailureLeavesStateUnchanged(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	diskFull := errors.New("disk full")
	h.persist.fail = diskFull
	if _, err := h.room.Act(t.Context(), prototest.Alice, 0, move("e4")); !errors.Is(err, diskFull) {
		t.Fatalf("want persist error, got %v", err)
	}
	if _, n := h.sent.last(prototest.Alice); n != 0 {
		t.Fatalf("failed step broadcast %d updates", n)
	}
	h.persist.setFail(nil)
	if got := h.play(t, prototest.Alice, 0, "e4"); got != 1 {
		t.Fatalf("retry seq %d, want 1", got)
	}
}

func TestFailedTimeoutPersistRetriesAfterDelay(t *testing.T) {
	h := newHarness(t, game.TimeControl10Plus5)
	h.play(t, prototest.Bob, h.play(t, prototest.Alice, 0, "e4"), "e5")
	h.persist.setFail(errors.New("disk full"))
	h.clock.Advance(10 * time.Minute)

	retryAt := h.clock.Now().Add(retryDelay)
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
	h.control(t, prototest.Bob, 0, protocol.ControlAbort)
	waitDone(t, h.room)
	final, _ := h.sent.last(prototest.Bob)
	for _, u := range []protocol.GameUpdate{h.snapshot(t, prototest.Alice), final} {
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
	h := newRelayHarness(t, game.TimeControlNone, prototest.Bob, prototest.Carol)
	seq := h.control(t, prototest.Bob, h.relay(t, prototest.Alice, 0, relayPass), protocol.ControlResign)
	if u, _ := h.sent.last(prototest.Bob); len(u.Controls) != 0 || u.Result != nil {
		t.Fatalf("resigned player: controls %v, result %+v; want no controls in a running game", u.Controls, u.Result)
	}
	for _, c := range []protocol.Control{protocol.ControlResign, protocol.ControlAbort} {
		_, err := h.room.Control(t.Context(), prototest.Bob, seq, c)
		prototest.WantCode(t, err, protocol.CodeIllegalAction)
	}
	_, before := h.sent.last(prototest.Bob)
	h.relay(t, prototest.Alice, h.relay(t, prototest.Carol, seq, relayPass), relayPass)
	if u, after := h.sent.last(prototest.Bob); after != before+2 || !slices.Equal(u.ToAct, []game.UserID{prototest.Carol}) {
		t.Fatalf("spectator got %d more updates, to_act %v; want 2 more and carol to act", after-before, u.ToAct)
	}
}

func TestClockStartsOnceRemainingPlayersActed(t *testing.T) {
	h := newRelayHarness(t, game.TimeControl10Plus5, prototest.Bob, prototest.Carol)
	seq := h.control(t, prototest.Bob, h.relay(t, prototest.Alice, 0, relayPass), protocol.ControlResign)
	h.relay(t, prototest.Carol, seq, relayPass)
	h.clock.Advance(time.Minute)
	u := h.snapshot(t, prototest.Alice)
	want := &protocol.WireClock{RemainingMS: map[game.UserID]int64{prototest.Alice: 540_000, prototest.Bob: 600_000, prototest.Carol: 600_000}, Running: new(prototest.Alice), AsOfMS: protocol.UnixMSOf(h.clock.Now())}
	if diff := cmp.Diff(want, u.Clock); diff != "" {
		t.Fatalf("clock after the remaining players acted (-want +got):\n%s", diff)
	}
	if slices.Contains(u.Controls, protocol.ControlAbort) {
		t.Fatalf("abort still offered after every remaining player acted: %v", u.Controls)
	}
}

func TestIncrementOncePerTurnHandover(t *testing.T) {
	h := newRelayHarness(t, game.TimeControl10Plus5, prototest.Bob)
	seq := h.relay(t, prototest.Bob, h.relay(t, prototest.Alice, 0, relayPass), relayPass)
	for _, m := range []relayMove{relayKeep, relayPass} {
		h.clock.Advance(10 * time.Second)
		seq = h.relay(t, prototest.Alice, seq, m)
	}
	want := &protocol.WireClock{RemainingMS: map[game.UserID]int64{prototest.Alice: 585_000, prototest.Bob: 600_000}, Running: new(prototest.Bob), AsOfMS: protocol.UnixMSOf(h.clock.Now())}
	if diff := cmp.Diff(want, h.snapshot(t, prototest.Alice).Clock); diff != "" {
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
			[]controlStep{{prototest.Bob, protocol.ControlResign}, {prototest.Alice, protocol.ControlOfferDraw}, {prototest.Carol, protocol.ControlOfferDraw}},
			game.ReasonDrawAgreed, map[game.UserID]store.Standing{prototest.Alice: draw, prototest.Bob: loss, prototest.Carol: draw},
		},
		{
			"holdout resigns after the others offered",
			[]controlStep{{prototest.Alice, protocol.ControlOfferDraw}, {prototest.Carol, protocol.ControlOfferDraw}, {prototest.Bob, protocol.ControlResign}},
			game.ReasonDrawAgreed, map[game.UserID]store.Standing{prototest.Alice: draw, prototest.Bob: loss, prototest.Carol: draw},
		},
		{
			"abort after a resign",
			[]controlStep{{prototest.Bob, protocol.ControlResign}, {prototest.Alice, protocol.ControlAbort}},
			game.ReasonAborted, map[game.UserID]store.Standing{prototest.Alice: none, prototest.Bob: loss, prototest.Carol: none},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRelayHarness(t, game.TimeControlNone, prototest.Bob, prototest.Carol)
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
	h := newRelayHarness(t, game.TimeControlNone, prototest.Bob, prototest.Carol)
	h.controls(t, 0, []controlStep{{prototest.Bob, protocol.ControlOfferDraw}, {prototest.Bob, protocol.ControlResign}})
	for _, user := range []game.UserID{prototest.Alice, prototest.Carol} {
		u := h.snapshot(t, user)
		if len(u.DrawAgreed) != 0 || slices.Contains(u.Controls, protocol.ControlDeclineDraw) {
			t.Errorf("%s: draw_agreed %v, controls %v; want no pending offer", user, u.DrawAgreed, u.Controls)
		}
	}
}

func TestEliminationEndingGameKeepsItsResultOverOpenOffers(t *testing.T) {
	h := newRelayHarness(t, game.TimeControlNone, prototest.Bob)
	h.controls(t, 0, []controlStep{{prototest.Alice, protocol.ControlOfferDraw}, {prototest.Bob, protocol.ControlResign}})
	finished := h.finished(t)
	want := map[game.UserID]store.Standing{prototest.Alice: {Outcome: store.OutcomeWin, Score: 1}, prototest.Bob: {Outcome: store.OutcomeLoss, Score: -1}}
	if finished.Reason != game.ReasonResign || !maps.Equal(finished.Standings, want) {
		t.Fatalf("finish %+v, want alice winning by resign", finished)
	}
}

func TestClockStartsWhenEliminationMeetsBarrier(t *testing.T) {
	h := newRelayHarness(t, game.TimeControl10Plus5, prototest.Bob, prototest.Carol)
	seq := h.relay(t, prototest.Bob, h.relay(t, prototest.Alice, 0, relayPass), relayPass)
	h.clock.Advance(time.Minute)
	h.control(t, prototest.Carol, seq, protocol.ControlResign)
	h.clock.Advance(time.Minute)
	want := &protocol.WireClock{RemainingMS: map[game.UserID]int64{prototest.Alice: 540_000, prototest.Bob: 600_000, prototest.Carol: 600_000}, Running: new(prototest.Alice), AsOfMS: protocol.UnixMSOf(h.clock.Now())}
	if diff := cmp.Diff(want, h.snapshot(t, prototest.Alice).Clock); diff != "" {
		t.Fatalf("clock a minute after the resign that met the barrier (-want +got):\n%s", diff)
	}
}

// timedOutOnOwnTurn plays a 3-player relay to the point where bob flags on
// his own turn, after alice has earned an increment.
func timedOutOnOwnTurn(t *testing.T) *harness {
	t.Helper()
	h := newRelayHarness(t, game.TimeControl10Plus5, prototest.Bob, prototest.Carol)
	seq := h.relay(t, prototest.Carol, h.relay(t, prototest.Bob, h.relay(t, prototest.Alice, 0, relayPass), relayPass), relayPass)
	h.clock.Advance(10 * time.Second)
	h.relay(t, prototest.Alice, seq, relayPass)
	h.clock.Advance(10 * time.Minute)
	return h
}

func TestTimeoutOnOwnTurnPassesClockToNextPlayer(t *testing.T) {
	h := timedOutOnOwnTurn(t)
	if controls := h.snapshot(t, prototest.Bob).Controls; len(controls) != 0 {
		t.Fatalf("timed-out player keeps controls %v", controls)
	}
	h.clock.Advance(time.Minute)
	want := &protocol.WireClock{RemainingMS: map[game.UserID]int64{prototest.Alice: 595_000, prototest.Bob: 0, prototest.Carol: 540_000}, Running: new(prototest.Carol), AsOfMS: protocol.UnixMSOf(h.clock.Now())}
	if diff := cmp.Diff(want, h.snapshot(t, prototest.Alice).Clock); diff != "" {
		t.Fatalf("clock a minute after bob flagged (-want +got):\n%s", diff)
	}
}

func TestRestoreAfterTimeoutRearmsTimer(t *testing.T) {
	h := timedOutOnOwnTurn(t)
	want := h.fullSnapshot(t, prototest.Bob)
	h.cancel()

	steps, _ := h.persist.recorded()
	restored, err := Restore(t.Context(), newSetup(t, h.params), steps)
	if err != nil {
		t.Fatal(err)
	}
	got, err := restored.Snapshot(t.Context(), prototest.Bob)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("bob's snapshot after restore (-want +got):\n%s", diff)
	}
	h.clock.Advance(10 * time.Minute)
	waitDone(t, restored)
	_, finished := h.persist.recorded()
	wantStandings := map[game.UserID]store.Standing{prototest.Alice: {Outcome: store.OutcomeWin, Score: 1}, prototest.Bob: {Outcome: store.OutcomeLoss, Score: -1}, prototest.Carol: {Outcome: store.OutcomeLoss, Score: -1}}
	if finished == nil || finished.Reason != game.ReasonTimeout || !maps.Equal(finished.Standings, wantStandings) {
		t.Fatalf("finish %+v, want carol flagging after restore to leave alice the winner", finished)
	}
}

func TestRestoreKeepsEliminatedPlayer(t *testing.T) {
	h := newRelayHarness(t, game.TimeControl10Plus5, prototest.Bob, prototest.Carol)
	seq := h.control(t, prototest.Bob, h.relay(t, prototest.Alice, 0, relayPass), protocol.ControlResign)
	seq = h.relay(t, prototest.Carol, seq, relayPass)
	users := []game.UserID{prototest.Alice, prototest.Bob, prototest.Carol}
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
	_, err = restored.Control(t.Context(), prototest.Bob, seq, protocol.ControlResign)
	prototest.WantCode(t, err, protocol.CodeIllegalAction)
}

func TestImposedResultLeavesNoMovesInTheFinalView(t *testing.T) {
	tests := map[string]func(h *harness){
		"draw agreed": func(h *harness) {
			seq := h.control(t, prototest.Alice, 0, protocol.ControlOfferDraw)
			h.control(t, prototest.Bob, seq, protocol.ControlOfferDraw)
		},
		"abort": func(h *harness) { h.control(t, prototest.Alice, 0, protocol.ControlAbort) },
	}
	for name, impose := range tests {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, game.TimeControl10Plus5)
			impose(h)
			waitDone(t, h.room)
			_, finished := h.persist.recorded()
			if finished == nil {
				t.Fatal("no finish record")
			}
			var view chess.View
			if err := json.Unmarshal(finished.Views[prototest.Alice], &view); err != nil {
				t.Fatal(err)
			}
			if len(view.LegalMoves) != 0 || view.ClaimableDraw {
				t.Errorf("stored final view of the side to move offers legal_moves %v, claimable_draw %v", view.LegalMoves, view.ClaimableDraw)
			}
		})
	}
}
