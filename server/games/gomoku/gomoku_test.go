package gomoku

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
	"github.com/shivamshivanshu/games.nvim/server/games/discs"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
)

// blackFiveOnRowEight has black take h8 to l8 while white fills the top row.
func blackFiveOnRowEight() []string {
	return []string{"h8", "a1", "i8", "b1", "j8", "c1", "k8", "d1", "l8"}
}

func coords() grid.Coords { return discs.NewBoard(boardSize).Coords() }

func place(cell string) discs.Action { return discs.Action{Kind: discs.ActionPlace, Cell: cell} }

func newGame(t *testing.T) game.State[discs.Action, View] {
	t.Helper()
	st, _, err := Rules{}.New(discs.Opts{First: game.FirstMoverMe}, alice, []game.UserID{bob}, game.StepRNG(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// places alternates the cells between the seats, black first.
func places(cells ...string) []gametest.Step[discs.Action] {
	steps := make([]gametest.Step[discs.Action], len(cells))
	seat := discs.SeatBlack
	for i, cell := range cells {
		steps[i] = gametest.Step[discs.Action]{Seat: seat, Action: place(cell)}
		seat = discs.Sides.Other(seat)
	}
	return steps
}

func play(t *testing.T, cells ...string) game.State[discs.Action, View] {
	t.Helper()
	st := newGame(t)
	for _, step := range places(cells...) {
		next, _, err := st.Apply(step.Seat, step.Action, nil)
		if err != nil {
			t.Fatalf("%s places %s: %v", step.Seat, step.Action.Cell, err)
		}
		st = next
	}
	return st
}

// drawPattern splits the board with no five anywhere: black takes each cell whose row
// plus half its column is even, 113 of them, and white the other 112.
func drawPattern() (black, white []string) {
	for row := range boardSize {
		for col := range boardSize {
			name := coords().Name(grid.Point{Row: row, Col: col})
			if (row+col/2)%2 == 0 {
				black = append(black, name)
			} else {
				white = append(white, name)
			}
		}
	}
	return black, white
}

// alternate interleaves the seats' cells, black first and last.
func alternate(black, white []string) []string {
	cells := make([]string, 0, len(black)+len(white))
	for i, b := range black {
		cells = append(cells, b)
		if i < len(white) {
			cells = append(cells, white[i])
		}
	}
	return cells
}

func fullBoardDraw() []string { return alternate(drawPattern()) }

// fullBoardFiveOnLastStone fills the board as fullBoardDraw does, except that black holds a8
// to e8 and gives j1, h12 and a13 to white, so its last stone, e8, the 225th on the board,
// makes the only five.
func fullBoardFiveOnLastStone() []string {
	black, white := drawPattern()
	five := []string{"a8", "b8", "c8", "d8", "e8"}
	given := []string{"j1", "h12", "a13"}
	isMoved := func(cell string) bool { return slices.Contains(five, cell) || slices.Contains(given, cell) }
	black = append(slices.DeleteFunc(black, isMoved), five...)
	white = append(slices.DeleteFunc(white, isMoved), given...)
	return alternate(black, white)
}

func cells(names ...string) []discs.Cell {
	out := make([]discs.Cell, len(names))
	for i, name := range names {
		p, ok := coords().Parse(name)
		if !ok {
			panic("bad cell " + name)
		}
		out[i] = discs.CellAt(p)
	}
	return out
}

func TestNewSeatsTheFirstMoverAsBlack(t *testing.T) {
	tests := []struct {
		first     game.FirstMover
		seed      game.Seed
		wantAlice game.Seat
	}{
		{game.FirstMoverMe, 1, discs.SeatBlack},
		{game.FirstMoverOpponent, 1, discs.SeatWhite},
		{game.FirstMoverRandom, 1, discs.SeatWhite},
		{game.FirstMoverRandom, 6, discs.SeatBlack},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s seed %d", tc.first, tc.seed), func(t *testing.T) {
			_, seats, err := Rules{}.New(discs.Opts{First: tc.first}, alice, []game.UserID{bob}, game.StepRNG(tc.seed, 0))
			if err != nil {
				t.Fatal(err)
			}
			if seats[alice] != tc.wantAlice || seats[bob] != discs.Sides.Other(tc.wantAlice) {
				t.Errorf("seats %v, want alice %s", seats, tc.wantAlice)
			}
		})
	}
}

func TestApplyRejects(t *testing.T) {
	tests := []struct {
		name    string
		played  []string
		seat    game.Seat
		action  discs.Action
		wantErr error
	}{
		{"column past o", nil, discs.SeatBlack, place("p1"), game.IllegalActionError{Msg: `cell must be a1 to o15, a letter then a number, got "p1"`}},
		{"row 0", nil, discs.SeatBlack, place("a0"), game.IllegalActionError{Msg: `cell must be a1 to o15, a letter then a number, got "a0"`}},
		{"row 16", nil, discs.SeatBlack, place("a16"), game.IllegalActionError{Msg: `cell must be a1 to o15, a letter then a number, got "a16"`}},
		{"no cell", nil, discs.SeatBlack, place(""), game.IllegalActionError{Msg: `cell must be a1 to o15, a letter then a number, got ""`}},
		{"number first", nil, discs.SeatBlack, place("8h"), game.IllegalActionError{Msg: `cell must be a1 to o15, a letter then a number, got "8h"`}},
		{"taken cell", []string{"h8"}, discs.SeatWhite, place("H8"), game.IllegalActionError{Msg: "h8 is taken"}},
		{"unknown kind", nil, discs.SeatBlack, discs.Action{Kind: "drop", Cell: "h8"}, game.IllegalActionError{Msg: "unknown action: drop"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := play(t, tc.played...).Apply(tc.seat, tc.action, nil)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestOutcome(t *testing.T) {
	blackWins := &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{discs.SeatBlack}, Reason: ReasonFiveInARow}
	tests := []struct {
		name      string
		played    []string
		want      *game.Result
		wantLines [][]discs.Cell
	}{
		{"four is not enough", blackFiveOnRowEight()[:7], nil, nil},
		{"row", blackFiveOnRowEight(), blackWins, [][]discs.Cell{cells("h8", "i8", "j8", "k8", "l8")}},
		{"column", []string{"a1", "b1", "a2", "b2", "a3", "b3", "a4", "b4", "a5"}, blackWins,
			[][]discs.Cell{cells("a1", "a2", "a3", "a4", "a5")}},
		{"diagonal", []string{"a1", "o1", "b2", "o2", "c3", "o3", "d4", "o4", "e5"}, blackWins,
			[][]discs.Cell{cells("a1", "b2", "c3", "d4", "e5")}},
		{"anti-diagonal on the far edge", []string{"o1", "a1", "n2", "a2", "m3", "a3", "l4", "a4", "k5"}, blackWins,
			[][]discs.Cell{cells("o1", "n2", "m3", "l4", "k5")}},
		{"white wins", []string{"h8", "a1", "h9", "a2", "h10", "a3", "h11", "a4", "o15", "a5"},
			&game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{discs.SeatWhite}, Reason: ReasonFiveInARow},
			[][]discs.Cell{cells("a1", "a2", "a3", "a4", "a5")}},
		{"an overline of six wins, reported whole", []string{"a8", "a1", "b8", "a2", "c8", "a3", "e8", "a4", "f8", "o15", "d8"}, blackWins,
			[][]discs.Cell{cells("a8", "b8", "c8", "d8", "e8", "f8")}},
		{"two lines at once", []string{"d8", "a1", "e8", "a2", "f8", "a3", "g8", "a4", "h4", "o1", "h5", "o2", "h6", "o3", "h7", "o4", "h8"}, blackWins,
			[][]discs.Cell{cells("d8", "e8", "f8", "g8", "h8"), cells("h4", "h5", "h6", "h7", "h8")}},
		{"a full board without five is a draw", fullBoardDraw(), &game.Result{Kind: game.ResultDraw, Reason: ReasonBoardFull}, nil},
		{"five with the 225th stone wins, not a full-board draw", fullBoardFiveOnLastStone(), blackWins,
			[][]discs.Cell{cells("a8", "b8", "c8", "d8", "e8")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := play(t, tc.played...)
			got, finished := st.Outcome()
			if finished != (tc.want != nil) {
				t.Fatalf("finished %t with %+v", finished, got)
			}
			if tc.want != nil {
				if diff := cmp.Diff(*tc.want, got); diff != "" {
					t.Errorf("result (-want +got):\n%s", diff)
				}
			}
			if diff := cmp.Diff(append([][]discs.Cell{}, tc.wantLines...), st.View(discs.SeatWhite).WinningLines); diff != "" {
				t.Errorf("winning lines (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCellTextIsTrimmedAndCaseless(t *testing.T) {
	st := play(t, " H8\t")
	if got := st.View(discs.SeatBlack).Last; got == nil || *got != (discs.Cell{Row: 8, Column: 8}) {
		t.Errorf("last %+v, want row 8 column 8", got)
	}
}

func TestCataloguePatternIsTheParsersOwn(t *testing.T) {
	spec := Rules{}.Info().Actions[0]
	if spec.Input.Pattern != coords().Pattern() {
		t.Errorf("catalogue pattern %s, want %s", spec.Input.Pattern, coords().Pattern())
	}
	if err := (Rules{}).Info().CheckActions(); err != nil {
		t.Error(err)
	}
}

func TestApplyLeavesEarlierStateUnchanged(t *testing.T) {
	before := play(t, "h8", "h9")
	want := before.View(discs.SeatBlack)
	if _, _, err := before.Apply(discs.SeatBlack, place("i8"), nil); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, before.View(discs.SeatBlack)); diff != "" {
		t.Errorf("earlier state changed (-before +after):\n%s", diff)
	}
}

func TestLegalCellsAreTheMoversOwn(t *testing.T) {
	concluded := play(t, "h8").(game.Concluder[discs.Action, View]).Conclude(game.Result{Kind: game.ResultDraw, Reason: game.ReasonDrawAgreed})
	tests := []struct {
		name         string
		state        game.State[discs.Action, View]
		black, white int
	}{
		{"start", play(t), boardSize * boardSize, 0},
		{"after black plays", play(t, "h8"), 0, boardSize*boardSize - 1},
		{"after a win", play(t, blackFiveOnRowEight()...), 0, 0},
		{"after an imposed result", concluded, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			black, white := tc.state.View(discs.SeatBlack).LegalCells, tc.state.View(discs.SeatWhite).LegalCells
			if len(black) != tc.black || len(white) != tc.white {
				t.Errorf("legal cells: black %d, white %d; want %d, %d", len(black), len(white), tc.black, tc.white)
			}
		})
	}
	if slices.Contains(play(t, "h8").View(discs.SeatWhite).LegalCells, (discs.Cell{Row: 8, Column: 8})) {
		t.Error("a taken cell is legal")
	}
}

func TestConcludeKeepsAReachedResult(t *testing.T) {
	imposed := game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	tests := []struct {
		name   string
		played []string
		want   game.Result
	}{
		{"ongoing takes the imposed result", []string{"h8"}, imposed},
		{"finished keeps its own", blackFiveOnRowEight(), game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{discs.SeatBlack}, Reason: ReasonFiveInARow}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, finished := play(t, tc.played...).(game.Concluder[discs.Action, View]).Conclude(imposed).Outcome()
			if diff := cmp.Diff(tc.want, got); !finished || diff != "" {
				t.Errorf("finished %t, result (-want +got):\n%s", finished, diff)
			}
		})
	}
}

func TestWireShape(t *testing.T) {
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(json.RawMessage(`{"first":"me","time_control":"none"}`))
	if err != nil {
		t.Fatal(err)
	}
	st, _, err := g.New(opts, alice, []game.UserID{bob}, game.StepRNG(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	st, events, err := st.Apply(discs.SeatBlack, json.RawMessage(`{"kind":"place","cell":"c2"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || string(events[0]) != `{"by":"black","cell":{"row":2,"column":3},"kind":"placed"}` {
		t.Errorf("events %s", events)
	}
	raw, err := st.View(discs.SeatWhite)
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]json.RawMessage
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	row := `["","","black","","","","","","","","","","","",""]`
	if got := string(view["cells"]); !strings.Contains(got, `[[`+strings.Repeat(`"",`, 14)+`""],`+row) {
		t.Errorf("cells %s", got)
	}
	if got := string(view["last"]); got != `{"row":2,"column":3}` {
		t.Errorf("last %s", got)
	}
	if got := string(view["winning_lines"]); got != `[]` {
		t.Errorf("winning_lines %s", got)
	}
	if got := string(view["legal_cells"]); !strings.HasPrefix(got, `[{"row":1,"column":1},`) {
		t.Errorf("legal_cells %.40s", got)
	}
}

func TestConformance(t *testing.T) {
	script := func(name string, steps []gametest.Step[discs.Action], want *game.Result) gametest.Script[discs.Opts, discs.Action] {
		return gametest.Script[discs.Opts, discs.Action]{
			Name:       name,
			Opts:       discs.Opts{First: game.FirstMoverRandom, TimeControl: game.TimeControlNone},
			Challenger: alice,
			Invitees:   []game.UserID{bob},
			Seed:       1,
			Steps:      steps,
			WantResult: want,
		}
	}
	taken := gametest.Step[discs.Action]{Seat: discs.SeatWhite, Action: place("h8"), WantErr: true}
	gametest.Run(t, Rules{}, []gametest.Script[discs.Opts, discs.Action]{
		script("black wins after a rejected move", slices.Insert(places(blackFiveOnRowEight()...), 1, taken),
			&game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{discs.SeatBlack}, Reason: ReasonFiveInARow}),
		script("draw", places(fullBoardDraw()...), &game.Result{Kind: game.ResultDraw, Reason: ReasonBoardFull}),
		script("ongoing", places("h8", "h9"), nil),
	})
}
