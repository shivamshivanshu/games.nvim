package discs

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

func TestBoardWithLeavesTheOriginalUnchanged(t *testing.T) {
	empty := NewBoard(3)
	placed := empty.With(SeatBlack, grid.Point{Row: 0, Col: 1}, grid.Point{Row: 2, Col: 2}).With(SeatWhite, grid.Point{Row: 1, Col: 0})
	if diff := cmp.Diff([][]game.Seat{{"", "", ""}, {"", "", ""}, {"", "", ""}}, empty.Rows()); diff != "" {
		t.Errorf("original changed (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([][]game.Seat{{"", "black", ""}, {"white", "", ""}, {"", "", "black"}}, placed.Rows()); diff != "" {
		t.Errorf("placed (-want +got):\n%s", diff)
	}
	if got := []int{placed.Count(SeatBlack), placed.Count(SeatWhite), len(placed.Empty())}; !cmp.Equal(got, []int{2, 1, 6}) {
		t.Errorf("black, white, empty = %v, want [2 1 6]", got)
	}
	if placed.Full() || !NewBoard(1).With(SeatWhite, grid.Point{}).Full() {
		t.Error("Full is not whether every cell holds a disc")
	}
	rows := placed.Rows()
	rows[0][0] = SeatWhite
	if placed.At(grid.Point{}) != NoDisc {
		t.Error("Rows shares the board's cells")
	}
}

func TestPlacedPoint(t *testing.T) {
	coords := NewBoard(8).Coords()
	tests := []struct {
		action  Action
		want    grid.Point
		wantErr error
	}{
		{Action{Kind: ActionPlace, Cell: "d3"}, grid.Point{Row: 2, Col: 3}, nil},
		{Action{Kind: ActionPlace, Cell: "i1"}, grid.Point{}, game.IllegalActionError{Msg: `cell must be a1 to h8, a letter then a number, got "i1"`}},
		{Action{Kind: "drop", Cell: "d3"}, grid.Point{}, game.IllegalActionError{Msg: "unknown action: drop"}},
	}
	for _, tc := range tests {
		got, err := PlacedPoint(coords, tc.action)
		if got != tc.want || !errors.Is(err, tc.wantErr) {
			t.Errorf("%+v: got %+v, %v; want %+v, %v", tc.action, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestLegalForIsTheMoversOnly(t *testing.T) {
	everywhere := func() []grid.Point { return []grid.Point{{Row: 0, Col: 0}} }
	tests := []struct {
		name      string
		turn      game.Turn
		wantLegal map[game.Seat][]grid.Cell
	}{
		{"black to move", game.TurnTo(SeatBlack), map[game.Seat][]grid.Cell{SeatBlack: {{Row: 1, Col: 1}}, SeatWhite: {}}},
		{"white to move", game.TurnTo(SeatWhite), map[game.Seat][]grid.Cell{SeatBlack: {}, SeatWhite: {{Row: 1, Col: 1}}}},
		{"finished", game.TurnTo(SeatBlack).Conclude(game.DrawBy("drawn")), map[game.Seat][]grid.Cell{SeatBlack: {}, SeatWhite: {}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for seat, want := range tc.wantLegal {
				if diff := cmp.Diff(want, LegalFor(tc.turn, seat, everywhere)); diff != "" {
					t.Errorf("%s legal (-want +got):\n%s", seat, diff)
				}
			}
		})
	}
}
