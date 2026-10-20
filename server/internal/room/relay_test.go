package room

import (
	"encoding/json"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

// relayRules seats any number of players who take turns in order. A turn
// lasts until its seat passes, so one turn can span several actions, and an
// eliminated seat drops out while the rest play on; the last seat left wins.
type relayRules struct{}

type relayMove string

const (
	relayKeep relayMove = "keep"
	relayPass relayMove = "pass"
)

type relayAction struct {
	Move relayMove `json:"move"`
}

type relayView struct {
	Turn   game.Seat `json:"turn"`
	Viewer game.Seat `json:"viewer"`
}

type relayState struct {
	order  []game.Seat
	turn   int
	result *game.Result
}

func (relayRules) Info() game.Info {
	return game.Info{
		Type:         "relay",
		Players:      game.PlayerRange{Min: 2, Max: 4},
		Capabilities: []game.Capability{game.CapResign, game.CapDraw, game.CapClock, game.CapAbort},
		Options:      []game.OptionSpec{game.TimeControlOption()},
	}
}

func (relayRules) New(_ struct{}, challenger game.UserID, invitees []game.UserID, _ *rand.Rand) (game.State[relayAction, relayView], map[game.UserID]game.Seat, error) {
	seats := map[game.UserID]game.Seat{}
	order := []game.Seat{}
	for _, user := range append([]game.UserID{challenger}, invitees...) {
		seats[user] = game.Seat(user)
		order = append(order, game.Seat(user))
	}
	return relayState{order: order}, seats, nil
}

func (s relayState) Apply(seat game.Seat, a relayAction, _ *rand.Rand) (game.State[relayAction, relayView], []game.Event, error) {
	switch {
	case s.result != nil:
		return nil, nil, game.IllegalActionError{Msg: "game is over"}
	case seat != s.order[s.turn]:
		return nil, nil, game.ErrNotYourTurn
	case a.Move != relayKeep && a.Move != relayPass:
		return nil, nil, game.IllegalActionError{Msg: "unknown move " + string(a.Move)}
	}
	if a.Move == relayPass {
		s.turn = (s.turn + 1) % len(s.order)
	}
	return s, nil, nil
}

func (s relayState) View(seat game.Seat) relayView {
	if s.result != nil {
		return relayView{Viewer: seat}
	}
	return relayView{Turn: s.order[s.turn], Viewer: seat}
}

func (s relayState) ToAct() []game.Seat {
	if s.result != nil {
		return []game.Seat{}
	}
	return []game.Seat{s.order[s.turn]}
}

func (s relayState) Outcome() (game.Result, bool) {
	if s.result == nil {
		return game.Result{}, false
	}
	return *s.result, true
}

func (s relayState) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[relayAction, relayView], []game.Event) {
	i := slices.Index(s.order, seat)
	if s.result != nil || i < 0 {
		return s, nil
	}
	s.order = slices.Delete(slices.Clone(s.order), i, i+1)
	if i < s.turn {
		s.turn--
	}
	s.turn %= len(s.order)
	if len(s.order) == 1 {
		s.result = &game.Result{Kind: game.ResultDecisive, Winners: slices.Clone(s.order), Reason: reason}
	}
	return s, nil
}

func newRelayHarness(t *testing.T, timeControl game.TimeControl, invitees ...game.UserID) *harness {
	t.Helper()
	return startHarness(t, game.Adapt(relayRules{}), `{"time_control":"`+string(timeControl)+`"}`, invitees...)
}

func (h *harness) relay(t *testing.T, user game.UserID, seq game.Seq, m relayMove) game.Seq {
	t.Helper()
	action, err := json.Marshal(relayAction{Move: m})
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
		Challenger: alice,
		Invitees:   []game.UserID{bob, carol},
		Steps: []gametest.Step[relayAction]{
			{Seat: game.Seat(alice), Action: relayAction{Move: relayKeep}},
			{Seat: game.Seat(alice), Action: relayAction{Move: relayPass}},
			{Seat: game.Seat(alice), Action: relayAction{Move: relayPass}, WantErr: true},
			{Seat: game.Seat(bob), Action: relayAction{Move: relayPass}},
		},
	}})
}
