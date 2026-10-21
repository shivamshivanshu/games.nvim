package discs

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// Turn is whose move it is and how the game ended, kept alike by both games.
type Turn struct {
	game.Ending
	toMove game.Seat
}

// FirstTurn is black to move.
func FirstTurn() Turn { return Turn{toMove: SeatBlack} }

// To is the turn passed to seat; a finished turn stays as it is.
func (t Turn) To(seat game.Seat) Turn {
	if t.Over() {
		return t
	}
	return Turn{toMove: seat}
}

// Conclude keeps a result already reached; otherwise result ends the game.
func (t Turn) Conclude(result game.Result) Turn {
	return Turn{Ending: t.Concluded(result), toMove: t.toMove}
}

// ToAct is the seat to move, none once finished.
func (t Turn) ToAct() []game.Seat {
	if t.Over() {
		return nil
	}
	return []game.Seat{t.toMove}
}

// LegalFor is the viewer's own legal cells: legal's, when the viewer is to move, else none.
func (t Turn) LegalFor(viewer game.Seat, legal func() []grid.Point) []Cell {
	if !slices.Contains(t.ToAct(), viewer) {
		return []Cell{}
	}
	return CellsAt(legal())
}
