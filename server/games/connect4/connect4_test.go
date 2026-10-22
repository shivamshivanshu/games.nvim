package connect4

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
)

const (
	fullBoardDraw   = "656173566152215676422337377473141445425321"
	winOnLastDisc   = "571376236345225146673541242612413735415776"
	columnOneFilled = "111111"
)

func newState(t *testing.T) game.State[Action, View] {
	t.Helper()
	st, _, err := Rules{}.New(game.TurnOpts{First: game.FirstMoverMe}, alice, []game.UserID{bob}, game.StepRNG(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// drops turns a string of column digits into alternating drops, red first.
func drops(columns string) []gametest.Step[Action] {
	steps := make([]gametest.Step[Action], len(columns))
	seat := SeatRed
	for i, digit := range columns {
		steps[i] = gametest.Step[Action]{Seat: seat, Action: drop(string(digit))}
		seat = sides.Other(seat)
	}
	return steps
}

func playColumns(t *testing.T, columns string) game.State[Action, View] {
	t.Helper()
	st := newState(t)
	for _, step := range drops(columns) {
		st, _ = gametest.MustApply(t, Rules{}.Info(), st, step.Seat, step.Action, nil)
	}
	return st
}

func line(cells ...grid.Cell) []grid.Cell { return cells }

func cell(row, col int) grid.Cell { return grid.Cell{Row: row, Col: col} }

func TestDropOutcomes(t *testing.T) {
	cases := []struct {
		name      string
		columns   string
		wantLast  grid.Cell
		wantLines [][]grid.Cell
		want      *game.Result
	}{
		{
			name:     "first disc lands on the bottom row",
			columns:  "4",
			wantLast: grid.Cell{Row: 6, Col: 4},
		},
		{
			name:     "second disc stacks on the first",
			columns:  "44",
			wantLast: grid.Cell{Row: 5, Col: 4},
		},
		{
			name:      "horizontal four",
			columns:   "1122334",
			wantLast:  grid.Cell{Row: 6, Col: 4},
			wantLines: [][]grid.Cell{line(cell(6, 1), cell(6, 2), cell(6, 3), cell(6, 4))},
			want:      &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatRed}, Reason: ReasonFourInARow},
		},
		{
			name:      "vertical four",
			columns:   "1212121",
			wantLast:  grid.Cell{Row: 3, Col: 1},
			wantLines: [][]grid.Cell{line(cell(3, 1), cell(4, 1), cell(5, 1), cell(6, 1))},
			want:      &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatRed}, Reason: ReasonFourInARow},
		},
		{
			name:      "rising diagonal",
			columns:   "12237334444",
			wantLast:  grid.Cell{Row: 3, Col: 4},
			wantLines: [][]grid.Cell{line(cell(3, 4), cell(4, 3), cell(5, 2), cell(6, 1))},
			want:      &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatRed}, Reason: ReasonFourInARow},
		},
		{
			name:      "falling diagonal",
			columns:   "76651554444",
			wantLast:  grid.Cell{Row: 3, Col: 4},
			wantLines: [][]grid.Cell{line(cell(3, 4), cell(4, 5), cell(5, 6), cell(6, 7))},
			want:      &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatRed}, Reason: ReasonFourInARow},
		},
		{
			name:      "a run of five is reported whole",
			columns:   "112244553",
			wantLast:  grid.Cell{Row: 6, Col: 3},
			wantLines: [][]grid.Cell{line(cell(6, 1), cell(6, 2), cell(6, 3), cell(6, 4), cell(6, 5))},
			want:      &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatRed}, Reason: ReasonFourInARow},
		},
		{
			name:     "two lines through the last disc",
			columns:  "152636576717714",
			wantLast: grid.Cell{Row: 6, Col: 4},
			wantLines: [][]grid.Cell{
				line(cell(6, 1), cell(6, 2), cell(6, 3), cell(6, 4)),
				line(cell(3, 7), cell(4, 6), cell(5, 5), cell(6, 4)),
			},
			want: &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatRed}, Reason: ReasonFourInARow},
		},
		{
			name:     "full board without four is a draw",
			columns:  fullBoardDraw,
			wantLast: grid.Cell{Row: 1, Col: 1},
			want:     &game.Result{Kind: game.ResultDraw, Reason: ReasonBoardFull},
		},
		{
			name:      "a win on the last empty cell beats the draw",
			columns:   winOnLastDisc,
			wantLast:  grid.Cell{Row: 1, Col: 6},
			wantLines: [][]grid.Cell{line(cell(1, 6), cell(2, 5), cell(3, 4), cell(4, 3))},
			want:      &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatYellow}, Reason: ReasonFourInARow},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := playColumns(t, tc.columns)
			view := st.View(SeatRed)
			if diff := cmp.Diff(&tc.wantLast, view.Last); diff != "" {
				t.Errorf("last (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(xslices.Clone(tc.wantLines), view.WinningLines); diff != "" {
				t.Errorf("winning lines (-want +got):\n%s", diff)
			}
			got, finished := st.Outcome()
			if tc.want == nil {
				if finished {
					t.Fatalf("finished early with %+v", got)
				}
				return
			}
			if diff := cmp.Diff(*tc.want, got); !finished || diff != "" {
				t.Errorf("finished=%t, result (-want +got):\n%s", finished, diff)
			}
		})
	}
}

func drop(column string) Action { return Action{Kind: ActionDrop, Column: column} }

func TestRejectedActions(t *testing.T) {
	cases := []struct {
		name    string
		columns string
		seat    game.Seat
		action  Action
		wantErr error
	}{
		{"full column", columnOneFilled, SeatRed, drop("1"), game.IllegalActionError{Msg: "column 1 is full"}},
		{"column below range", "", SeatRed, drop("0"), game.IllegalActionError{Msg: `column must be 1 to 7, got "0"`}},
		{"column above range", "", SeatRed, drop("8"), game.IllegalActionError{Msg: `column must be 1 to 7, got "8"`}},
		{"column missing", "", SeatRed, drop(""), game.IllegalActionError{Msg: `column must be 1 to 7, got ""`}},
		{"column not a number", "", SeatRed, drop("a"), game.IllegalActionError{Msg: `column must be 1 to 7, got "a"`}},
		{"column padded", "", SeatRed, drop(" 3 "), game.IllegalActionError{Msg: `column must be 1 to 7, got " 3 "`}},
		{"unknown kind", "", SeatRed, Action{Kind: "place", Column: "1"}, game.IllegalActionError{Msg: "unknown action: place"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := playColumns(t, tc.columns)
			_, _, err := st.Apply(tc.seat, tc.action, nil)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestApplyLeavesEarlierStateUnchanged(t *testing.T) {
	before := playColumns(t, "44")
	want := before.View(SeatRed)
	if _, _, err := before.Apply(SeatRed, drop("4"), nil); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, before.View(SeatRed)); diff != "" {
		t.Errorf("earlier state changed (-before +after):\n%s", diff)
	}
}

func TestLegalColumns(t *testing.T) {
	concluded := playColumns(t, "4").Conclude(game.Result{Kind: game.ResultDraw, Reason: game.ReasonDrawAgreed})
	cases := []struct {
		name   string
		state  game.State[Action, View]
		viewer game.Seat
		want   []int
	}{
		{"all open for the mover", newState(t), SeatRed, []int{1, 2, 3, 4, 5, 6, 7}},
		{"none for the waiting seat", newState(t), SeatYellow, []int{}},
		{"a full column is left out", playColumns(t, columnOneFilled), SeatRed, []int{2, 3, 4, 5, 6, 7}},
		{"none once won", playColumns(t, "1122334"), SeatYellow, []int{}},
		{"none after an imposed result", concluded, SeatYellow, []int{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, tc.state.View(tc.viewer).LegalColumns); diff != "" {
				t.Errorf("legal columns (-want +got):\n%s", diff)
			}
		})
	}
	if result, finished := concluded.Outcome(); !finished || result.Reason != game.ReasonDrawAgreed || len(concluded.ToAct()) != 0 {
		t.Errorf("concluded: %+v finished=%t to act %v", result, finished, concluded.ToAct())
	}
}

func TestConcludeKeepsAWinAlreadyReached(t *testing.T) {
	won := playColumns(t, "1122334")
	want, _ := won.Outcome()
	after := won.Conclude(game.Result{Kind: game.ResultDraw, Reason: game.ReasonDrawAgreed})
	got, finished := after.Outcome()
	if diff := cmp.Diff(want, got); !finished || diff != "" {
		t.Errorf("finished=%t, result (-want +got):\n%s", finished, diff)
	}
}

func TestFirstMoverIsRed(t *testing.T) {
	cases := []struct {
		name  string
		first game.FirstMover
		seed  game.Seed
		want  game.Seat
	}{
		{"me", game.FirstMoverMe, 1, SeatRed},
		{"opponent", game.FirstMoverOpponent, 1, SeatYellow},
		{"random, seed 1", game.FirstMoverRandom, 1, SeatYellow},
		{"random, seed 6", game.FirstMoverRandom, 6, SeatRed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, seats, err := Rules{}.New(game.TurnOpts{First: tc.first}, alice, []game.UserID{bob}, game.StepRNG(tc.seed, 0))
			if err != nil {
				t.Fatal(err)
			}
			if seats[alice] != tc.want || seats[bob] != sides.Other(tc.want) {
				t.Errorf("seats %v, want alice %s", seats, tc.want)
			}
			if diff := cmp.Diff([]game.Seat{SeatRed}, st.ToAct()); diff != "" {
				t.Errorf("to act (-want +got):\n%s", diff)
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
	emptyRow := `["","","","","","",""]`
	start := `{"cells":[` + strings.Repeat(emptyRow+`,`, 5) + emptyRow + `],"last":null,"winning_lines":[],"legal_columns":[1,2,3,4,5,6,7]}`
	assertJSON(t, "start view", start, viewJSON(t, st, SeatRed))

	st, events, err := st.Apply(SeatRed, json.RawMessage(`{"kind":"drop","column":"3"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	assertJSON(t, "dropped event", `{"kind":"dropped","by":"red","cell":{"row":6,"col":3}}`, events[0])
	var view View
	if err := json.Unmarshal(viewJSON(t, st, SeatYellow), &view); err != nil {
		t.Fatal(err)
	}
	if view.Cells[5][2] != SeatRed || *view.Last != (grid.Cell{Row: 6, Col: 3}) {
		t.Errorf("after drop: cells row 6 %v, last %+v", view.Cells[5], view.Last)
	}
}

func viewJSON(t *testing.T, st game.ErasedState, seat game.Seat) json.RawMessage {
	t.Helper()
	raw, err := st.View(seat)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertJSON(t *testing.T, what, want string, got json.RawMessage) {
	t.Helper()
	var wantDoc, gotDoc any
	if err := json.Unmarshal([]byte(want), &wantDoc); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &gotDoc); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantDoc, gotDoc); diff != "" {
		t.Errorf("%s (-want +got):\n%s", what, diff)
	}
}

func TestConformance(t *testing.T) {
	script := func(name string, steps []gametest.Step[Action], want game.Result) gametest.Script[game.TurnOpts, Action] {
		return gametest.Script[game.TurnOpts, Action]{
			Name:       name,
			Opts:       game.TurnOpts{First: game.FirstMoverMe, TimeControl: game.TimeControlNone},
			Challenger: alice,
			Invitees:   []game.UserID{bob},
			Seed:       1,
			Steps:      steps,
			WantResult: &want,
		}
	}
	intoFullColumn := gametest.Step[Action]{Seat: SeatRed, Action: drop("1"), WantErr: true}
	redVerticalInColumnTwo := drops("2324252")
	redWins := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatRed}, Reason: ReasonFourInARow}
	gametest.Run(t, Rules{}, []gametest.Script[game.TurnOpts, Action]{
		script("full column rejected, then vertical four", slices.Concat(drops(columnOneFilled), []gametest.Step[Action]{intoFullColumn}, redVerticalInColumnTwo), redWins),
		script("full board draw", drops(fullBoardDraw), game.Result{Kind: game.ResultDraw, Reason: ReasonBoardFull}),
		script("win on the last disc", drops(winOnLastDisc), game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatYellow}, Reason: ReasonFourInARow}),
	})
}
