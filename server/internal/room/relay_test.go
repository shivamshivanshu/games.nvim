package room

import (
	"encoding/json"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/prototest"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

// relayRules seats any number of players who take turns in order. A turn
// lasts until its seat passes, so one turn can span several actions, and an
// eliminated seat drops out while the rest play on; the last seat left wins.
// A seat that bows out leaves by the rules, as a game.Knockout.
type relayRules struct{}

type relayMove string

const (
	relayKeep relayMove = "keep"
	relayPass relayMove = "pass"
	relayOut  relayMove = "out"

	reasonLastLeft game.ResultReason = "last_left"

	relayMoveKind game.ActionKind = "move"
)

type relayAction struct {
	Kind game.ActionKind `json:"kind"`
	Move relayMove       `json:"move"`
}

type relayView struct {
	Turn   game.Seat `json:"turn"`
	Viewer game.Seat `json:"viewer"`
}

type relayState struct {
	game.Ending
	order []game.Seat
	out   []game.Seat
	turn  int
}

func (relayRules) Info() game.Info {
	return game.Info{
		Type:         "relay",
		Players:      game.PlayerRange{Min: 2, Max: 4},
		Capabilities: []game.Capability{game.CapResign, game.CapDraw, game.CapClock, game.CapAbort},
		Options:      []game.OptionSpec{game.TimeControlOption()},
		Actions:      []game.ActionSpec{{Kind: relayMoveKind, Description: "Keep the turn, pass it, or bow out."}},
	}
}

func (relayRules) New(_ struct{}, challenger game.UserID, invitees []game.UserID, _ *rand.Rand) (game.State[relayAction, relayView], map[game.UserID]game.Seat, error) {
	seats := map[game.UserID]game.Seat{}
	order := []game.Seat{}
	for _, user := range game.Participants(challenger, invitees) {
		seats[user] = game.Seat(user)
		order = append(order, game.Seat(user))
	}
	return relayState{order: order}, seats, nil
}

func (s relayState) Apply(seat game.Seat, a relayAction, _ *rand.Rand) (game.State[relayAction, relayView], []game.Event, error) {
	switch {
	case s.Over():
		return nil, nil, game.IllegalActionError{Msg: "game is over"}
	case seat != s.order[s.turn]:
		return nil, nil, game.ErrNotYourTurn
	case a.Move != relayKeep && a.Move != relayPass && a.Move != relayOut:
		return nil, nil, game.IllegalActionError{Msg: "unknown move " + string(a.Move)}
	}
	if a.Move == relayOut {
		next := s.without(seat, reasonLastLeft)
		next.out = append(slices.Clone(s.out), seat)
		return next, nil, nil
	}
	if a.Move == relayPass {
		s.turn = (s.turn + 1) % len(s.order)
	}
	return s, nil, nil
}

func (s relayState) View(seat game.Seat) relayView {
	if s.Over() {
		return relayView{Viewer: seat}
	}
	return relayView{Turn: s.order[s.turn], Viewer: seat}
}

func (s relayState) ToAct() []game.Seat {
	if s.Over() {
		return []game.Seat{}
	}
	return []game.Seat{s.order[s.turn]}
}

func (s relayState) KnockedOut() []game.Seat { return slices.Clone(s.out) }

func (s relayState) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[relayAction, relayView], []game.Event) {
	if s.Over() || !slices.Contains(s.order, seat) {
		return s, nil
	}
	return s.without(seat, reason), nil
}

func (s relayState) without(seat game.Seat, reason game.ResultReason) relayState {
	i := slices.Index(s.order, seat)
	s.order = slices.Delete(slices.Clone(s.order), i, i+1)
	if i < s.turn {
		s.turn--
	}
	s.turn %= len(s.order)
	if len(s.order) == 1 {
		s.Ending = s.Concluded(game.WinFor(reason, s.order...))
	}
	return s
}

func newRelayHarness(t *testing.T, timeControl game.TimeControl, invitees ...game.UserID) *harness {
	t.Helper()
	return startHarness(t, game.Adapt(relayRules{}), `{"time_control":"`+string(timeControl)+`"}`, invitees...)
}

func (h *harness) relay(t *testing.T, user game.UserID, seq game.Seq, m relayMove) game.Seq {
	t.Helper()
	action, err := json.Marshal(relayAction{Kind: relayMoveKind, Move: m})
	if err != nil {
		t.Fatal(err)
	}
	next, err := h.room.Act(t.Context(), user, seq, action)
	if err != nil {
		t.Fatalf("%s %s at seq %d: %v", user, m, seq, err)
	}
	return next
}

func TestRelayConforms(t *testing.T) {
	gametest.Run(t, relayRules{}, []gametest.Script[struct{}, relayAction]{{
		Name:       "three seats",
		Challenger: prototest.Alice,
		Invitees:   []game.UserID{prototest.Bob, prototest.Carol},
		Steps: []gametest.Step[relayAction]{
			{Seat: game.Seat(prototest.Alice), Action: relayAction{Kind: relayMoveKind, Move: relayKeep}},
			{Seat: game.Seat(prototest.Alice), Action: relayAction{Kind: relayMoveKind, Move: relayPass}},
			{Seat: game.Seat(prototest.Alice), Action: relayAction{Kind: relayMoveKind, Move: relayPass}, WantErr: true},
			{Seat: game.Seat(prototest.Bob), Action: relayAction{Kind: relayMoveKind, Move: relayPass}},
		},
	}})
}

func TestKnockedOutSeatLosesItsControlsAndAnImposedEnding(t *testing.T) {
	h := newRelayHarness(t, game.TimeControlNone, prototest.Bob, prototest.Carol)
	seq := h.relay(t, prototest.Alice, 0, relayOut)
	if u := h.snapshot(t, prototest.Alice); len(u.Controls) != 0 {
		t.Fatalf("knocked-out seat's controls %v, want none", u.Controls)
	}
	for _, c := range []protocol.Control{protocol.ControlResign, protocol.ControlAbort} {
		_, err := h.room.Control(t.Context(), prototest.Alice, seq, c)
		prototest.WantCode(t, err, protocol.CodeIllegalAction)
	}
	h.control(t, prototest.Bob, seq, protocol.ControlAbort)
	loss, none := store.Standing{Outcome: store.OutcomeLoss, Score: -1}, store.Standing{Outcome: store.OutcomeNone}
	want := map[game.UserID]store.Standing{prototest.Alice: loss, prototest.Bob: none, prototest.Carol: none}
	if diff := cmp.Diff(want, h.finished(t).Standings); diff != "" {
		t.Fatalf("standings after an abort (-want +got):\n%s", diff)
	}
}

func TestRestoreKeepsKnockedOutSeatWithoutControls(t *testing.T) {
	h := newRelayHarness(t, game.TimeControlNone, prototest.Bob, prototest.Carol)
	seq := h.relay(t, prototest.Alice, 0, relayOut)
	h.cancel()

	steps, _ := h.persist.recorded()
	restored, err := Restore(t.Context(), newSetup(t, h.params), steps)
	if err != nil {
		t.Fatal(err)
	}
	_, err = restored.Control(t.Context(), prototest.Alice, seq, protocol.ControlAbort)
	prototest.WantCode(t, err, protocol.CodeIllegalAction)
}
