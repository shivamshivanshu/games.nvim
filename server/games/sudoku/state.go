package sudoku

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type state struct {
	game.Ending
	puzzle, grid, solution Grid
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
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
		s.Ending = s.Concluded(game.WinFor(ReasonSolved, game.SeatSolo))
	}
	return s, []game.Event{CellSetEvent{By: game.SeatSolo, Row: set.row, Col: set.col, Digit: set.digit}}, nil
}

func (s state) View(game.Seat) View {
	view := View{Grid: s.grid, Conflicts: conflicts(s.grid), Solution: []Digit{}}
	for cell, d := range s.puzzle {
		view.Given[cell] = d != Empty
	}
	if s.Over() {
		view.Solution = s.solution[:]
	}
	return view
}

func (s state) ToAct() []game.Seat {
	if s.Over() {
		return nil
	}
	return []game.Seat{game.SeatSolo}
}

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining([]game.Seat{game.SeatSolo}, seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game and the view reveals the solution.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Ending = s.Concluded(result)
	return s
}
