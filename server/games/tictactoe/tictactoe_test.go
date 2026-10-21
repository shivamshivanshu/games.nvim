package tictactoe

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
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
		st, _ = gametest.MustApply(t, st, st.ToAct()[0], place(c), game.StepRNG(1, 1))
	}
	return st
}

func place(c Cell) Action { return Action{Kind: ActionPlace, Cell: strconv.Itoa(int(c))} }

func TestNewSeatsFirstMoverAsX(t *testing.T) {
	tests := []struct {
		first     game.FirstMover
		seed      game.Seed
		wantAlice game.Seat
	}{
		{game.FirstMoverMe, 1, SeatX},
		{game.FirstMoverOpponent, 1, SeatO},
		{game.FirstMoverRandom, 1, SeatO},
		{game.FirstMoverRandom, 6, SeatX},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s seed %d", tc.first, tc.seed), func(t *testing.T) {
			for range 2 {
				_, seats, err := Rules{}.New(Opts{First: tc.first}, alice, []game.UserID{bob}, game.StepRNG(tc.seed, 0))
				if err != nil {
					t.Fatal(err)
				}
				if seats[alice] != tc.wantAlice || seats[bob] != sides.Other(tc.wantAlice) {
					t.Errorf("seats %v, want alice %s", seats, tc.wantAlice)
				}
			}
		})
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
		{"cell 0", nil, SeatX, place(0), game.IllegalActionError{Msg: `cell must be 1-9, got "0"`}},
		{"cell 10", nil, SeatX, place(10), game.IllegalActionError{Msg: `cell must be 1-9, got "10"`}},
		{"no cell", nil, SeatX, Action{Kind: ActionPlace}, game.IllegalActionError{Msg: `cell must be 1-9, got ""`}},
		{"signed cell", nil, SeatX, Action{Kind: ActionPlace, Cell: "+5"}, game.IllegalActionError{Msg: `cell must be 1-9, got "+5"`}},
		{"taken cell", []Cell{5}, SeatO, place(5), game.IllegalActionError{Msg: "cell 5 is taken"}},
		{"unknown kind", nil, SeatX, Action{Kind: "move", Cell: "1"}, game.IllegalActionError{Msg: "unknown action: move"}},
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

func TestApplyLeavesEarlierStateUnchanged(t *testing.T) {
	before := play(t, 1, 4, 2, 5)
	want := before.View(SeatX)
	if _, _, err := before.Apply(SeatX, place(3), game.StepRNG(1, 1)); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, before.View(SeatX)); diff != "" {
		t.Errorf("earlier state changed (-before +after):\n%s", diff)
	}
	if _, finished := before.Outcome(); finished {
		t.Error("earlier state finished")
	}
}

func TestLegalCellsAreTheMoversOwn(t *testing.T) {
	imposed := game.Result{Kind: game.ResultDraw, Reason: game.ReasonDrawAgreed}
	tests := []struct {
		name         string
		state        game.State[Action, View]
		wantX, wantO []Cell
	}{
		{"start", play(t), []Cell{1, 2, 3, 4, 5, 6, 7, 8, 9}, []Cell{}},
		{"after X plays 5", play(t, 5), []Cell{}, []Cell{1, 2, 3, 4, 6, 7, 8, 9}},
		{"after a win", play(t, 1, 4, 2, 5, 3), []Cell{}, []Cell{}},
		{"after an imposed result", play(t, 5).Conclude(imposed), []Cell{}, []Cell{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.wantX, tc.state.View(SeatX).LegalCells); diff != "" {
				t.Errorf("x (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantO, tc.state.View(SeatO).LegalCells); diff != "" {
				t.Errorf("o (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConcludeKeepsAReachedResult(t *testing.T) {
	imposed := game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	tests := []struct {
		name   string
		played []Cell
		want   game.Result
	}{
		{"ongoing takes the imposed result", []Cell{1}, imposed},
		{"finished keeps its own", []Cell{1, 4, 2, 5, 3}, game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatX}, Reason: ReasonThreeInARow}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := play(t, tc.played...).Conclude(imposed)
			got, finished := st.Outcome()
			if !finished || len(st.ToAct()) != 0 {
				t.Fatalf("finished %t, to act %v", finished, st.ToAct())
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("result (-want +got):\n%s", diff)
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
	wantView(t, st, `{"cells":["","","","","","","","",""],"last":null,"winning_lines":[],"legal_cells":[]}`)
	st, events, err := st.Apply(SeatX, json.RawMessage(`{"kind":"place","cell":"3"}`), game.StepRNG(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || string(events[0]) != `{"kind":"placed","by":"x","cell":3}` {
		t.Errorf("events %s", events)
	}
	wantView(t, st, `{"cells":["","","x","","","","","",""],"last":3,"winning_lines":[],"legal_cells":[1,2,4,5,6,7,8,9]}`)
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
		steps[i] = gametest.Step[Action]{Seat: seat, Action: place(c)}
		seat = sides.Other(seat)
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
	taken := gametest.Step[Action]{Seat: SeatO, Action: place(1), WantErr: true}
	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		script("x wins after a rejected move", slices.Insert(places(1, 4, 2, 5, 3), 1, taken),
			&game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatX}, Reason: ReasonThreeInARow}),
		script("draw", places(1, 2, 3, 5, 4, 6, 8, 7, 9), &game.Result{Kind: game.ResultDraw, Reason: ReasonBoardFull}),
		script("ongoing", places(5, 1), nil),
	})
}
