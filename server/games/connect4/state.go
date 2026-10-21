package connect4

import (
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

type state struct {
	game.Ending
	board        board
	toMove       game.Seat
	last         *grid.Cell
	winningLines [][]grid.Cell
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	switch action.Kind {
	case ActionDrop:
		return s.drop(seat, action.Column)
	}
	return nil, nil, game.UnknownAction(action.Kind)
}

func (s state) drop(seat game.Seat, input string) (game.State[Action, View], []game.Event, error) {
	column, ok := game.ParseNumbered(input, boardColumns)
	if !ok {
		return nil, nil, game.Illegalf("column must be 1 to %d, got %q", boardColumns, input)
	}
	row, ok := s.board.landingRow(column - 1)
	if !ok {
		return nil, nil, game.Illegalf("column %d is full", column)
	}
	at := grid.Point{Row: row, Col: column - 1}
	landed := at.Cell()
	next := s
	next.board[at.Row][at.Col] = seat
	next.last = &landed
	next.toMove = sides.Other(seat)
	next.winningLines = grid.CellLines(grid.WinningLines(next.board.cells(), at, winLength, noDisc))
	switch {
	case len(next.winningLines) > 0:
		next.Ending = next.Concluded(game.WinFor(ReasonFourInARow, seat))
	case next.board.full():
		next.Ending = next.Concluded(game.DrawBy(ReasonBoardFull))
	}
	return next, []game.Event{DroppedEvent{By: seat, Cell: landed}}, nil
}

func (s state) View(viewer game.Seat) View {
	legal := []int{}
	if slices.Contains(s.ToAct(), viewer) {
		legal = s.board.openColumns()
	}
	return View{
		Cells:        s.board.cells(),
		Last:         s.last,
		WinningLines: s.winningLines,
		LegalColumns: legal,
	}
}

func (s state) ToAct() []game.Seat {
	if s.Over() {
		return nil
	}
	return []game.Seat{s.toMove}
}

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining(sides.Seats(), seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game, so no seat is
// offered a drop.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Ending = s.Concluded(result)
	return s
}
