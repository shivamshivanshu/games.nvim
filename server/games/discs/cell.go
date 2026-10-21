package discs

import (
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

// Cell is a board position on the wire, 1-based: row 1 is the top, column 1 the left.
type Cell struct {
	Row    int `json:"row"`
	Column int `json:"column"`
}

// CellAt is the wire cell of a 0-based point.
func CellAt(p grid.Point) Cell { return Cell{Row: p.Row + 1, Column: p.Col + 1} }

// CellsAt maps points to wire cells, in order; it is never nil.
func CellsAt(points []grid.Point) []Cell {
	return xslices.Map(points, CellAt)
}
