package sudoku

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const solo game.UserID = "alice"

func wikipedia(t testing.TB) state {
	t.Helper()
	puzzle := gridOf(t, wikipediaPuzzle)
	return state{puzzle: puzzle, grid: puzzle, solution: gridOf(t, wikipediaSolution)}
}

func set(text string) Action { return Action{Kind: ActionSet, Text: text} }

func TestParseSet(t *testing.T) {
	tests := []struct {
		text string
		want cellSet
	}{
		{"53 7", cellSet{row: 5, col: 3, digit: 7}},
		{"  11\t9 ", cellSet{row: 1, col: 1, digit: 9}},
		{"99 0", cellSet{row: 9, col: 9, digit: Empty}},
	}
	for _, tt := range tests {
		got, err := parseSet(tt.text)
		if err != nil || got != tt.want {
			t.Errorf("parseSet(%q) = %+v, %v; want %+v", tt.text, got, err, tt.want)
		}
	}
	for _, text := range []string{"", "537", "5 3 7", "53", "03 7", "50 7", "53 x", "a3 7", "53 10", "153 7", "53 -"} {
		_, err := parseSet(text)
		want := `want "RC D", e.g. "53 7" for row 5, column 3, digit 7 (0 clears); got "` + text + `"`
		if !errors.As(err, new(game.IllegalActionError)) || err.Error() != want {
			t.Errorf("parseSet(%q): got %v, want illegal action %q", text, err, want)
		}
	}
}

func TestSetWritesAndClearsCells(t *testing.T) {
	tests := []struct {
		name      string
		before    []string
		action    string
		wantCell  Digit
		wantEvent CellSetEvent
	}{
		{"set an empty cell", nil, "13 4", 4, CellSetEvent{By: game.SeatSolo, Row: 1, Col: 3, Digit: 4}},
		{"a wrong digit is accepted", nil, "13 9", 9, CellSetEvent{By: game.SeatSolo, Row: 1, Col: 3, Digit: 9}},
		{"overwrite a set cell", []string{"13 9"}, "13 4", 4, CellSetEvent{By: game.SeatSolo, Row: 1, Col: 3, Digit: 4}},
		{"clear a set cell", []string{"13 4"}, "13 0", Empty, CellSetEvent{By: game.SeatSolo, Row: 1, Col: 3, Digit: Empty}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := wikipedia(t)
			for _, text := range tt.before {
				s, _ = gametest.MustApply(t, Rules{}.Info(), s, game.SeatSolo, set(text), nil)
			}
			next, events := gametest.MustApply(t, Rules{}.Info(), s, game.SeatSolo, set(tt.action), nil)
			if diff := cmp.Diff([]game.Event{tt.wantEvent}, events); diff != "" {
				t.Errorf("events (-want +got):\n%s", diff)
			}
			view := next.View(game.SeatSolo)
			if got := view.Grid[cellIndex(1, 3)]; got != tt.wantCell {
				t.Errorf("cell 1,3 = %d, want %d", got, tt.wantCell)
			}
			if _, finished := next.Outcome(); finished || len(view.Solution) != 0 {
				t.Errorf("finished=%t solution=%v; want an unfinished game with the solution hidden", finished, view.Solution)
			}
		})
	}
}

func TestIllegalSets(t *testing.T) {
	filled, _ := gametest.MustApply(t, Rules{}.Info(), wikipedia(t), game.SeatSolo, set("13 4"), nil)
	tests := []struct {
		name    string
		state   state
		action  Action
		wantMsg string
	}{
		{"overwrite a given", wikipedia(t), set("11 4"), "cell 1,1 is a given"},
		{"clear a given", wikipedia(t), set("11 0"), "cell 1,1 is a given"},
		{"rewrite the same digit", filled, set("13 4"), "cell 1,3 already holds 4"},
		{"clear an empty cell", wikipedia(t), set("13 0"), "cell 1,3 is already empty"},
		{"malformed text", wikipedia(t), set("1 3 4"), `want "RC D", e.g. "53 7" for row 5, column 3, digit 7 (0 clears); got "1 3 4"`},
		{"unknown kind", wikipedia(t), Action{Kind: "erase"}, "unknown action: erase"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tt.state.Apply(game.SeatSolo, tt.action, nil)
			if !errors.As(err, new(game.IllegalActionError)) || err.Error() != tt.wantMsg {
				t.Errorf("got %v, want illegal action %q", err, tt.wantMsg)
			}
		})
	}
}

func lastCellOpen(t *testing.T) state {
	t.Helper()
	s := wikipedia(t)
	s.grid = s.solution
	s.grid[cellIndex(9, 1)] = Empty
	return s
}

func TestMatchingTheSolutionSolvesAndRevealsIt(t *testing.T) {
	next, _ := gametest.MustApply(t, Rules{}.Info(), lastCellOpen(t), game.SeatSolo, set("91 3"), nil)
	result, finished := next.Outcome()
	want := game.WinFor(ReasonSolved, game.SeatSolo)
	if diff := cmp.Diff(want, result); !finished || diff != "" {
		t.Errorf("finished=%t, result (-want +got):\n%s", finished, diff)
	}
	if len(next.ToAct()) != 0 {
		t.Errorf("ToAct = %v after solving, want none", next.ToAct())
	}
	if diff := cmp.Diff(next.solution[:], next.View(game.SeatSolo).Solution); diff != "" {
		t.Errorf("solution after finishing (-want +got):\n%s", diff)
	}
}

func TestAFullGridThatDiffersFromTheSolutionPlaysOn(t *testing.T) {
	next, _ := gametest.MustApply(t, Rules{}.Info(), lastCellOpen(t), game.SeatSolo, set("91 8"), nil)
	if _, finished := next.Outcome(); finished {
		t.Error("a full grid with a wrong digit finished the game")
	}
	if fixed, _ := gametest.MustApply(t, Rules{}.Info(), next, game.SeatSolo, set("91 3"), nil); len(fixed.ToAct()) != 0 {
		t.Error("correcting the wrong digit did not solve the game")
	}
}

func TestGivingUpLosesAndRevealsTheSolution(t *testing.T) {
	st, events := wikipedia(t).Eliminate(game.SeatSolo, game.ReasonResign)
	result, finished := st.Outcome()
	want := game.LossBy(game.ReasonResign)
	if diff := cmp.Diff(want, result); !finished || len(events) != 0 || diff != "" {
		t.Errorf("finished=%t events=%v, result (-want +got):\n%s", finished, events, diff)
	}
	if got := st.View(game.SeatSolo).Solution; len(got) != CellCount {
		t.Errorf("solution after giving up = %v, want all %d cells", got, CellCount)
	}
}

func TestApplyAndEliminateLeaveTheEarlierStateUnchanged(t *testing.T) {
	tests := []struct {
		name string
		s    state
		step func(state)
	}{
		{"set", wikipedia(t), func(s state) { gametest.MustApply(t, Rules{}.Info(), s, game.SeatSolo, set("13 4"), nil) }},
		{"solving set", lastCellOpen(t), func(s state) { gametest.MustApply(t, Rules{}.Info(), s, game.SeatSolo, set("91 3"), nil) }},
		{"eliminate", wikipedia(t), func(s state) { s.Eliminate(game.SeatSolo, game.ReasonResign) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := tt.s
			tt.step(tt.s)
			if diff := cmp.Diff(before, tt.s, cmp.AllowUnexported(state{}, game.Ending{})); diff != "" {
				t.Errorf("earlier state changed (-before +after):\n%s", diff)
			}
		})
	}
}

func TestNewRejectsAnUnknownDifficulty(t *testing.T) {
	if _, _, err := (Rules{}).New(Opts{Difficulty: "expert"}, solo, nil, game.StepRNG(1, 0)); !errors.As(err, new(game.BadRequestError)) {
		t.Errorf("got %v, want a bad request", err)
	}
}

func TestEveryDifficultyReachesTheGeneratorThroughTheAdapter(t *testing.T) {
	tests := []struct {
		raw        string
		wantGivens int
	}{
		{`{}`, 32},
		{`{"difficulty":"easy"}`, 40},
		{`{"difficulty":"medium"}`, 32},
		{`{"difficulty":"hard"}`, 26},
	}
	g := game.Adapt(Rules{})
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			opts, err := g.ResolveOptions(json.RawMessage(tt.raw))
			if err != nil {
				t.Fatal(err)
			}
			st, seats, err := g.New(opts, solo, nil, game.StepRNG(1, 0))
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(map[game.UserID]game.Seat{solo: game.SeatSolo}, seats); diff != "" {
				t.Errorf("seats (-want +got):\n%s", diff)
			}
			raw, err := st.View(game.SeatSolo)
			if err != nil {
				t.Fatal(err)
			}
			var view View
			if err := json.Unmarshal(raw, &view); err != nil {
				t.Fatal(err)
			}
			if givens := countGivens(view.Grid); givens != tt.wantGivens {
				t.Errorf("%d givens, want %d", givens, tt.wantGivens)
			}
		})
	}
}

func TestConflictsMarkEveryCellWhoseDigitRepeatsInAUnit(t *testing.T) {
	tests := []struct {
		name  string
		cells map[int]Digit
		want  []int
	}{
		{"row", map[int]Digit{cellIndex(1, 1): 5, cellIndex(1, 9): 5}, []int{cellIndex(1, 1), cellIndex(1, 9)}},
		{"column", map[int]Digit{cellIndex(2, 4): 3, cellIndex(8, 4): 3}, []int{cellIndex(2, 4), cellIndex(8, 4)}},
		{"box", map[int]Digit{cellIndex(4, 4): 7, cellIndex(6, 6): 7}, []int{cellIndex(4, 4), cellIndex(6, 6)}},
		{"three in a row, one also in a column", map[int]Digit{cellIndex(1, 1): 2, cellIndex(1, 5): 2, cellIndex(9, 5): 2}, []int{cellIndex(1, 1), cellIndex(1, 5), cellIndex(9, 5)}},
		{"same digit sharing no unit", map[int]Digit{cellIndex(1, 1): 5, cellIndex(2, 4): 5}, nil},
		{"different digits in a row", map[int]Digit{cellIndex(1, 1): 5, cellIndex(1, 2): 6}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var g Grid
			for cell, d := range tt.cells {
				g[cell] = d
			}
			var want [CellCount]bool
			for _, cell := range tt.want {
				want[cell] = true
			}
			if diff := cmp.Diff(want, conflicts(g)); diff != "" {
				t.Errorf("conflicts (-want +got):\n%s", diff)
			}
		})
	}
}

func TestViewMarksAnEntryThatRepeatsAGiven(t *testing.T) {
	next, _ := gametest.MustApply(t, Rules{}.Info(), wikipedia(t), game.SeatSolo, set("13 5"), nil)
	view := next.View(game.SeatSolo)
	var want [CellCount]bool
	want[cellIndex(1, 1)], want[cellIndex(1, 3)] = true, true
	if diff := cmp.Diff(want, view.Conflicts); diff != "" {
		t.Errorf("conflicts (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([CellCount]bool{}, wikipedia(t).View(game.SeatSolo).Conflicts); diff != "" {
		t.Errorf("the puzzle alone has conflicts (-want +got):\n%s", diff)
	}
}

func TestConcludeEndsAnUnfinishedGameAndKeepsAReachedResult(t *testing.T) {
	aborted := game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	st := wikipedia(t).Conclude(aborted)
	if result, finished := st.Outcome(); !finished || !cmp.Equal(aborted, result) || len(st.ToAct()) != 0 {
		t.Errorf("finished=%t result=%+v to_act=%v; want the imposed abort with nobody to act", finished, result, st.ToAct())
	}
	if got := st.View(game.SeatSolo).Solution; len(got) != CellCount {
		t.Errorf("solution after an imposed end = %v, want all %d cells", got, CellCount)
	}
	solved, _ := gametest.MustApply(t, Rules{}.Info(), lastCellOpen(t), game.SeatSolo, set("91 3"), nil)
	if result, _ := solved.Conclude(aborted).Outcome(); result.Reason != ReasonSolved {
		t.Errorf("Conclude replaced a reached result with %+v", result)
	}
}

func TestViewMarksExactlyThePuzzlesCellsAsGiven(t *testing.T) {
	var want [CellCount]bool
	for cell, char := range wikipediaPuzzle {
		want[cell] = char != '.'
	}
	filled, _ := gametest.MustApply(t, Rules{}.Info(), wikipedia(t), game.SeatSolo, set("13 4"), nil)
	if diff := cmp.Diff(want, filled.View(game.SeatSolo).Given); diff != "" {
		t.Errorf("given (-want +got):\n%s", diff)
	}
}
