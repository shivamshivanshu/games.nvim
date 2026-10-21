package discs

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// Turn is whose move it is and how the game ended, kept alike by both games.
type Turn struct {
	toMove   game.Seat
	result   game.Result
	finished bool
}

// FirstTurn is black to move.
func FirstTurn() Turn { return Turn{toMove: SeatBlack} }

// To is the turn passed to seat; a finished turn stays as it is.
func (t Turn) To(seat game.Seat) Turn {
	if t.finished {
		return t
	}
	return Turn{toMove: seat}
}

// Finish ends the game with result.
func (t Turn) Finish(result game.Result) Turn {
	return Turn{toMove: t.toMove, result: result, finished: true}
}

// Conclude keeps a result already reached; otherwise result ends the game.
func (t Turn) Conclude(result game.Result) Turn {
	if t.finished {
		return t
	}
	return t.Finish(result)
}

// ToAct is the seat to move, none once finished.
func (t Turn) ToAct() []game.Seat {
	if t.finished {
		return nil
	}
	return []game.Seat{t.toMove}
}

// Outcome is the result, once finished.
func (t Turn) Outcome() (game.Result, bool) { return t.result, t.finished }

// LegalFor is the viewer's own legal cells: legal's, when the viewer is to move, else none.
func (t Turn) LegalFor(viewer game.Seat, legal func() []grid.Point) []Cell {
	if !slices.Contains(t.ToAct(), viewer) {
		return []Cell{}
	}
	return CellsAt(legal())
}
