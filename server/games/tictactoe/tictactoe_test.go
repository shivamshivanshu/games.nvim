package tictactoe

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
)

func newGame(t *testing.T) game.State[Action, View] {
	t.Helper()
	st, _, err := Rules{}.New(Opts{First: game.FirstMoverMe}, alice, []game.UserID{bob}, game.StepRNG(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func play(t *testing.T, cells ...Cell) game.State[Action, View] {
	t.Helper()
	st := newGame(t)
	for _, c := range cells {
		next, _, err := st.Apply(st.ToAct()[0], Action{Kind: ActionPlace, Cell: c}, game.StepRNG(1, 1))
		if err != nil {
			t.Fatalf("place %d: %v", c, err)
		}
		st = next
	}
	return st
}

func TestNewSeatsFirstMoverAsX(t *testing.T) {
	tests := []struct {
		first     game.FirstMover
		wantAlice game.Seat
	}{
		{game.FirstMoverMe, SeatX},
		{game.FirstMoverOpponent, SeatO},
	}
	for _, tc := range tests {
		t.Run(string(tc.first), func(t *testing.T) {
			_, seats, err := Rules{}.New(Opts{First: tc.first}, alice, []game.UserID{bob}, game.StepRNG(1, 0))
			if err != nil {
				t.Fatal(err)
			}
			if seats[alice] != tc.wantAlice || seats[bob] != opponent(tc.wantAlice) {
				t.Errorf("seats %v, want alice %s", seats, tc.wantAlice)
			}
		})
	}
}

func TestNewNeedsExactlyOneOpponent(t *testing.T) {
	for _, invitees := range [][]game.UserID{nil, {bob, "carol"}} {
		_, _, err := Rules{}.New(Opts{}, alice, invitees, game.StepRNG(1, 0))
		if !errors.As(err, &game.BadRequestError{}) {
			t.Errorf("%d invitees: got %v", len(invitees), err)
		}
	}
}

func TestApplyRejects(t *testing.T) {
	tests := []struct {
		name    string
		played  []Cell
		seat    game.Seat
		action  Action
		wantErr error
	}{
		{"cell 0", nil, SeatX, Action{Kind: ActionPlace, Cell: 0}, game.IllegalActionError{Msg: "cell must be 1-9, got 0"}},
		{"cell 10", nil, SeatX, Action{Kind: ActionPlace, Cell: 10}, game.IllegalActionError{Msg: "cell must be 1-9, got 10"}},
		{"taken cell", []Cell{5}, SeatO, Action{Kind: ActionPlace, Cell: 5}, game.IllegalActionError{Msg: "cell 5 is taken"}},
		{"unknown kind", nil, SeatX, Action{Kind: "move", Cell: 1}, game.IllegalActionError{Msg: "unknown action: move"}},
		{"O moves first", nil, SeatO, Action{Kind: ActionPlace, Cell: 1}, game.ErrNotYourTurn},
		{"X moves twice", []Cell{1}, SeatX, Action{Kind: ActionPlace, Cell: 2}, game.ErrNotYourTurn},
		{"after a win", []Cell{1, 4, 2, 5, 3}, SeatO, Action{Kind: ActionPlace, Cell: 6}, game.ErrNotYourTurn},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := play(t, tc.played...).Apply(tc.seat, tc.action, game.StepRNG(1, 1))
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestOutcome(t *testing.T) {
	xWins := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatX}, Reason: ReasonThreeInARow}
	tests := []struct {
		name      string
		played    []Cell
		want      game.Result
		finished  bool
		wantLines [][]Cell
	}{
		{"ongoing", []Cell{1, 4, 2, 5}, game.Result{}, false, [][]Cell{}},
		{"row", []Cell{1, 4, 2, 5, 3}, xWins, true, [][]Cell{{1, 2, 3}}},
		{"column", []Cell{1, 2, 4, 3, 7}, xWins, true, [][]Cell{{1, 4, 7}}},
		{"diagonal", []Cell{1, 2, 5, 3, 9}, xWins, true, [][]Cell{{1, 5, 9}}},
		{"O on the anti-diagonal", []Cell{1, 5, 2, 3, 9, 7},
			game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatO}, Reason: ReasonThreeInARow}, true, [][]Cell{{3, 5, 7}}},
		{"win on a full board beats the draw", []Cell{1, 4, 2, 5, 6, 8, 7, 9, 3}, xWins, true, [][]Cell{{1, 2, 3}}},
		{"two lines at once", []Cell{2, 5, 3, 6, 4, 8, 7, 9, 1}, xWins, true, [][]Cell{{1, 2, 3}, {1, 4, 7}}},
		{"draw", []Cell{1, 2, 3, 5, 4, 6, 8, 7, 9}, game.Result{Kind: game.ResultDraw, Reason: ReasonBoardFull}, true, [][]Cell{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := play(t, tc.played...)
			got, finished := st.Outcome()
			if finished != tc.finished {
				t.Errorf("finished %t, want %t", finished, tc.finished)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("result (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantLines, st.View(SeatO).WinningLines); diff != "" {
				t.Errorf("winning lines (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWireShape(t *testing.T) {
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(json.RawMessage(`{"first":"me"}`))
	if err != nil {
		t.Fatal(err)
	}
	st, _, err := g.New(opts, alice, []game.UserID{bob}, game.StepRNG(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	wantView(t, st, `{"cells":["","","","","","","","",""],"last":null,"winning_lines":[]}`)
	st, events, err := st.Apply(SeatX, json.RawMessage(`{"kind":"place","cell":3}`), game.StepRNG(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || string(events[0]) != `{"by":"x","cell":3,"kind":"placed"}` {
		t.Errorf("events %s", events)
	}
	wantView(t, st, `{"cells":["","","x","","","","","",""],"last":3,"winning_lines":[]}`)
}

func wantView(t *testing.T, st game.ErasedState, want string) {
	t.Helper()
	got, err := st.View(SeatO)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("view %s, want %s", got, want)
	}
}

func places(cells ...Cell) []gametest.Step[Action] {
	steps := make([]gametest.Step[Action], len(cells))
	seat := SeatX
	for i, c := range cells {
		steps[i] = gametest.Step[Action]{Seat: seat, Action: Action{Kind: ActionPlace, Cell: c}}
		seat = opponent(seat)
	}
	return steps
}

func TestConformance(t *testing.T) {
	script := func(name string, steps []gametest.Step[Action], want *game.Result) gametest.Script[Opts, Action] {
		return gametest.Script[Opts, Action]{
			Name:       name,
			Opts:       Opts{First: game.FirstMoverRandom},
			Challenger: alice,
			Invitees:   []game.UserID{bob},
			Seed:       1,
			Steps:      steps,
			WantResult: want,
		}
	}
	taken := gametest.Step[Action]{Seat: SeatO, Action: Action{Kind: ActionPlace, Cell: 1}, WantErr: true}
	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		script("x wins after a rejected move", slices.Insert(places(1, 4, 2, 5, 3), 1, taken),
			&game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatX}, Reason: ReasonThreeInARow}),
		script("draw", places(1, 2, 3, 5, 4, 6, 8, 7, 9), &game.Result{Kind: game.ResultDraw, Reason: ReasonBoardFull}),
		script("ongoing", places(5, 1), nil),
	})
}
