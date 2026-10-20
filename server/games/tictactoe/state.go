package tictactoe

import (
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

var allSeats = []game.Seat{SeatX, SeatO}

func newState() state {
	return state{toMove: SeatX, winningLines: [][]Cell{}}
}

func opponent(seat game.Seat) game.Seat {
	if seat == SeatX {
		return SeatO
	}
	return SeatX
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if !slices.Contains(s.ToAct(), seat) {
		return nil, nil, game.ErrNotYourTurn
	}
	if action.Kind != ActionPlace {
		return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
	}
	if err := s.board.placeError(action.Cell); err != nil {
		return nil, nil, err
	}
	return s.place(seat, action.Cell), []game.Event{PlacedEvent{By: seat, Cell: action.Cell}}, nil
}

func (s state) place(seat game.Seat, cell Cell) state {
	next := state{board: s.board.with(cell, seat), toMove: opponent(seat), last: cell}
	next.winningLines = next.board.winningLinesThrough(cell)
	switch {
	case len(next.winningLines) > 0:
		next.result = game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat}, Reason: ReasonThreeInARow}
		next.finished = true
	case next.board.full():
		next.result, next.finished = game.Result{Kind: game.ResultDraw, Reason: ReasonBoardFull}, true
	}
	return next
}

func (s state) View(game.Seat) View {
	view := View{Cells: s.board, WinningLines: s.winningLines}
	if s.last != noCell {
		last := s.last
		view.Last = &last
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
	if s.finished {
		return s, nil
	}
	s.result, s.finished = game.ForfeitToRemaining(allSeats, seat, reason), true
	return s, nil
}
