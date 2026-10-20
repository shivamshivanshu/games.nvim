package sudoku

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type state struct {
	puzzle, grid, solution Grid
	result                 game.Result
	finished               bool
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if s.finished || seat != SeatSolver {
		return nil, nil, game.ErrNotYourTurn
	}
	if action.Kind != ActionSet {
		return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
	}
	set, err := parseSet(action.Text)
	if err != nil {
		return nil, nil, err
	}
	return s.applySet(set)
}

func (s state) applySet(set cellSet) (game.State[Action, View], []game.Event, error) {
	cell := cellIndex(set.row, set.col)
	switch {
	case s.puzzle[cell] != Empty:
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("cell %d,%d is a given", set.row, set.col)}
	case s.grid[cell] == set.digit && set.digit == Empty:
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("cell %d,%d is already empty", set.row, set.col)}
	case s.grid[cell] == set.digit:
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("cell %d,%d already holds %d", set.row, set.col, set.digit)}
	}
	s.grid[cell] = set.digit
	if s.grid == s.solution {
		s.result = game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonSolved}
		s.finished = true
	}
	return s, []game.Event{CellSetEvent{By: SeatSolver, Row: set.row, Col: set.col, Digit: set.digit}}, nil
}

func (s state) View(game.Seat) View {
	view := View{Grid: s.grid, Solution: []Digit{}}
	for cell, d := range s.puzzle {
		view.Given[cell] = d != Empty
	}
	if s.finished {
		view.Solution = s.solution[:]
	}
	return view
}

func (s state) ToAct() []game.Seat {
	if s.finished {
		return nil
	}
	return []game.Seat{SeatSolver}
}

func (s state) Outcome() (game.Result, bool) { return s.result, s.finished }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	if s.finished {
		return s, nil
	}
	s.result, s.finished = game.ForfeitToRemaining([]game.Seat{SeatSolver}, seat, reason), true
	return s, nil
}
