package tictactoe

import (
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

const noCell CellNumber = 0

type state struct {
	game.Turn
	board        board
	last         CellNumber
	winningLines [][]grid.Cell
}

func newState() state {
	return state{Turn: game.TurnTo(SeatX), winningLines: [][]grid.Cell{}}
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if action.Kind != ActionPlace {
		return nil, nil, game.UnknownAction(action.Kind)
	}
	cell, err := parseCell(action.Cell)
	if err != nil {
		return nil, nil, err
	}
	if s.board.owner(cell) != noSeat {
		return nil, nil, game.Illegalf("cell %d is taken", cell)
	}
	return s.place(seat, cell), []game.Event{PlacedEvent{By: seat, Cell: cell}}, nil
}

func (s state) place(seat game.Seat, cell CellNumber) state {
	next := state{board: s.board.with(cell, seat), last: cell}
	next.winningLines = next.board.winningLinesThrough(cell)
	switch {
	case len(next.winningLines) > 0:
		next.Turn = s.Turn.Conclude(game.WinFor(ReasonThreeInARow, seat))
	case next.board.full():
		next.Turn = s.Turn.Conclude(game.DrawBy(ReasonBoardFull))
	default:
		next.Turn = s.To(sides.Other(seat))
	}
	return next
}

func (s state) View(seat game.Seat) View {
	view := View{Cells: s.board, WinningLines: s.winningLines, LegalCells: []CellNumber{}}
	if s.last != noCell {
		last := s.last.point().Cell()
		view.Last = &last
	}
	if slices.Contains(s.ToAct(), seat) {
		view.LegalCells = s.board.emptyCells()
	}
	return view
}

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining(sides.Seats(), seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Turn = s.Turn.Conclude(result)
	return s
}
