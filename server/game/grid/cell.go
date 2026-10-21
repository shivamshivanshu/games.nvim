package grid

import "github.com/shivamshivanshu/games.nvim/server/internal/xslices"

// Cell is a board position on the wire, 1-based: row 1 is the top, col 1 the left.
type Cell struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// Cell is p's wire cell.
func (p Point) Cell() Cell { return Cell{Row: p.Row + 1, Col: p.Col + 1} }

// Cells maps points to wire cells, in order; it is never nil.
func Cells(points []Point) []Cell { return xslices.Map(points, Point.Cell) }

// CellLines maps lines of points, such as WinningLines gives, to wire cells; it is never nil.
func CellLines(lines [][]Point) [][]Cell { return xslices.Map(lines, Cells) }
