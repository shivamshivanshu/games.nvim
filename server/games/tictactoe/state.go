package tictactoe

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const noCell Cell = 0

type state struct {
	board        board
	toMove       game.Seat
	last         Cell
	winningLines [][]Cell
	result       game.Result
	finished     bool
}

func newState() state {
	return state{toMove: SeatX, winningLines: [][]Cell{}}
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if action.Kind != ActionPlace {
		return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
	}
	cell, err := parseCell(action.Cell)
	if err != nil {
		return nil, nil, err
	}
	if s.board.owner(cell) != noSeat {
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("cell %d is taken", cell)}
	}
	return s.place(seat, cell), []game.Event{PlacedEvent{By: seat, Cell: cell}}, nil
}

func (s state) place(seat game.Seat, cell Cell) state {
	next := state{board: s.board.with(cell, seat), toMove: sides.Other(seat), last: cell}
	next.winningLines = next.board.winningLinesThrough(cell)
	switch {
	case len(next.winningLines) > 0:
		next.result = game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat}, Reason: ReasonThreeInARow}
		next.finished = true
	case len(next.board.emptyCells()) == 0:
		next.result, next.finished = game.Result{Kind: game.ResultDraw, Reason: ReasonBoardFull}, true
	}
	return next
}

func (s state) View(seat game.Seat) View {
	view := View{Cells: s.board, WinningLines: s.winningLines, LegalCells: []Cell{}}
	if s.last != noCell {
		last := s.last
		view.Last = &last
	}
	if slices.Contains(s.ToAct(), seat) {
		view.LegalCells = s.board.emptyCells()
	}
	return view
}

func (s state) ToAct() []game.Seat {
	if s.finished {
		return nil
	}
	return []game.Seat{s.toMove}
}

func (s state) Outcome() (game.Result, bool) { return s.result, s.finished }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining(sides.Seats(), seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	if s.finished {
		return s
	}
	s.result, s.finished = result, true
	return s
}
