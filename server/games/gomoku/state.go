package gomoku

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
	"github.com/shivamshivanshu/games.nvim/server/games/discs"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

type state struct {
	discs.Turn
	board        discs.Board
	last         *discs.Cell
	winningLines [][]discs.Cell
}

func newState() state {
	return state{Turn: discs.FirstTurn(), board: discs.NewBoard(boardSize), winningLines: [][]discs.Cell{}}
}

func (s state) Apply(seat game.Seat, action discs.Action, _ *rand.Rand) (game.State[discs.Action, View], []game.Event, error) {
	at, err := discs.PlacedPoint(s.board.Coords(), action)
	if err != nil {
		return nil, nil, err
	}
	if s.board.At(at) != discs.NoDisc {
		return nil, nil, game.Illegalf("%s is taken", s.board.Coords().Name(at))
	}
	next := s.place(seat, at)
	return next, []game.Event{PlacedEvent{By: seat, Cell: *next.last}}, nil
}

func (s state) place(seat game.Seat, at grid.Point) state {
	last := discs.CellAt(at)
	next := state{board: s.board.With(seat, at), last: &last}
	next.winningLines = xslices.Map(grid.WinningLines(next.board.Rows(), at, winLength, discs.NoDisc), discs.CellsAt)
	switch {
	case len(next.winningLines) > 0:
		next.Turn = s.Turn.Conclude(game.WinFor(ReasonFiveInARow, seat))
	case len(next.board.Empty()) == 0:
		next.Turn = s.Turn.Conclude(game.DrawBy(ReasonBoardFull))
	default:
		next.Turn = s.To(discs.Sides.Other(seat))
	}
	return next
}

func (s state) View(viewer game.Seat) View {
	return View{
		Cells:        s.board.Rows(),
		Last:         s.last,
		WinningLines: s.winningLines,
		LegalCells:   s.LegalFor(viewer, s.board.Empty),
	}
}

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[discs.Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining(discs.Sides.Seats(), seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game, so no seat is
// offered a cell.
func (s state) Conclude(result game.Result) game.State[discs.Action, View] {
	s.Turn = s.Turn.Conclude(result)
	return s
}
