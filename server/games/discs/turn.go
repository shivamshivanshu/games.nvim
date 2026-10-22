package discs

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// LegalFor is the viewer's own legal cells: legal's, when the viewer is to move, else none.
func LegalFor(turn game.Turn, viewer game.Seat, legal func() []grid.Point) []grid.Cell {
	if !slices.Contains(turn.ToAct(), viewer) {
		return []grid.Cell{}
	}
	return grid.Cells(legal())
}
