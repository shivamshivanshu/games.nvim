package discs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
)

func TestSeatPlayersGivesBlackToTheFirstMover(t *testing.T) {
	tests := []struct {
		first     game.FirstMover
		seed      game.Seed
		wantAlice game.Seat
	}{
		{game.FirstMoverMe, 1, SeatBlack},
		{game.FirstMoverOpponent, 1, SeatWhite},
		{game.FirstMoverRandom, 1, SeatWhite},
		{game.FirstMoverRandom, 6, SeatBlack},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s seed %d", tc.first, tc.seed), func(t *testing.T) {
			seats, err := SeatPlayers("gomoku", tc.first, alice, []game.UserID{bob}, game.StepRNG(tc.seed, 0))
			if err != nil {
				t.Fatal(err)
			}
			if seats[alice] != tc.wantAlice || seats[bob] != Opponent(tc.wantAlice) {
				t.Errorf("seats %v, want alice %s", seats, tc.wantAlice)
			}
		})
	}
}

func TestSeatPlayersNeedsExactlyOneOpponent(t *testing.T) {
	for _, invitees := range [][]game.UserID{nil, {bob, "carol"}} {
		_, err := SeatPlayers("reversi", game.FirstMoverMe, alice, invitees, game.StepRNG(1, 0))
		want := game.BadRequestError{Msg: fmt.Sprintf("reversi needs exactly one opponent, got %d", len(invitees))}
		if !errors.Is(err, want) {
			t.Errorf("%d invitees: got %v", len(invitees), err)
		}
	}
}

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

func TestTurn(t *testing.T) {
	won := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatBlack}, Reason: "won"}
	imposed := game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	everywhere := func() []grid.Point { return []grid.Point{{Row: 0, Col: 0}} }
	tests := []struct {
		name       string
		turn       Turn
		wantToAct  []game.Seat
		wantResult game.Result
		wantLegal  map[game.Seat][]Cell
	}{
		{"black first", FirstTurn(), []game.Seat{SeatBlack}, game.Result{},
			map[game.Seat][]Cell{SeatBlack: {{Row: 1, Column: 1}}, SeatWhite: {}}},
		{"passed to white", FirstTurn().To(SeatWhite), []game.Seat{SeatWhite}, game.Result{},
			map[game.Seat][]Cell{SeatBlack: {}, SeatWhite: {{Row: 1, Column: 1}}}},
		{"concluded", FirstTurn().Conclude(imposed), nil, imposed, map[game.Seat][]Cell{SeatBlack: {}, SeatWhite: {}}},
		{"a reached result outlives a conclusion", FirstTurn().Finish(won).Conclude(imposed), nil, won,
			map[game.Seat][]Cell{SeatBlack: {}, SeatWhite: {}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, finished := tc.turn.Outcome()
			if finished != (tc.wantToAct == nil) || !cmp.Equal(tc.wantResult, result) {
				t.Errorf("outcome %+v, %t", result, finished)
			}
			if diff := cmp.Diff(tc.wantToAct, tc.turn.ToAct()); diff != "" {
				t.Errorf("to act (-want +got):\n%s", diff)
			}
			for seat, want := range tc.wantLegal {
				if diff := cmp.Diff(want, tc.turn.LegalFor(seat, everywhere)); diff != "" {
					t.Errorf("%s legal (-want +got):\n%s", seat, diff)
				}
			}
		})
	}
}
