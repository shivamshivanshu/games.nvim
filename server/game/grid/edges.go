package grid

import (
	"fmt"
	"regexp"
)

// EdgeOrientation is the way an edge runs from its dot: across to the dot on its right, or down
// to the dot below.
type EdgeOrientation string

// The orientations.
const (
	EdgeAcross EdgeOrientation = "across"
	EdgeDown   EdgeOrientation = "down"
)

// Edge is the line between two adjacent dots, the corners of a board's cells. Dot is its left
// end lying across or its top end lying down, indexed like a cell from 0 at the top left on the
// lattice of dots, which is one row and one column larger than the cells.
type Edge struct {
	Dot         Point
	Orientation EdgeOrientation
}

// Edges names the edges of a board of cells: an edge is its dot's name, as Coords names the
// cells of a board one row and one column larger, then h lying across or v lying down, as in b2h.
type Edges struct {
	rows, cols int
	dots       Coords
	edge       *regexp.Regexp
}

const (
	acrossSuffix = "h"
	downSuffix   = "v"
)

// The submatches of the edge pattern: a dot's column and row lying across, or lying down.
const (
	acrossColumn = 1 + iota
	acrossRow
	downColumn
	downRow
)

// NewEdges names the edges of a board of rows by cols cells, which must be 1-18 by 1-25.
func NewEdges(rows, cols int) Edges {
	if rows < 1 || rows >= maxCoordRows || cols < 1 || cols >= maxCoordCols {
		panic(fmt.Sprintf("grid: no edges for %dx%d cells, want 1-%d rows and 1-%d columns", rows, cols, maxCoordRows-1, maxCoordCols-1))
	}
	across := `([` + columnClass(cols) + `])(` + rowAlternatives(rows+1) + `)[hH]`
	down := `([` + columnClass(cols+1) + `])(` + rowAlternatives(rows) + `)[vV]`
	return Edges{
		rows: rows,
		cols: cols,
		dots: NewCoords(rows+1, cols+1),
		edge: regexp.MustCompile(`^\s*(?:` + across + `|` + down + `)\s*$`),
	}
}

// Pattern is the RE2 expression of the text Parse accepts: a dot's column letter and row number
// as Coords takes them, then h or v in either case, naming only edges on the board.
func (e Edges) Pattern() string { return e.edge.String() }

// Parse reads the edge text names, reporting false for text Pattern does not match.
func (e Edges) Parse(text string) (Edge, bool) {
	match := e.edge.FindStringSubmatch(text)
	switch {
	case match == nil:
		return Edge{}, false
	case match[acrossColumn] != "":
		return Edge{Dot: pointNamed(match[acrossColumn], match[acrossRow]), Orientation: EdgeAcross}, true
	}
	return Edge{Dot: pointNamed(match[downColumn], match[downRow]), Orientation: EdgeDown}, true
}

// Name is the edge's text, in lower case: Parse(Name(edge)) is edge.
func (e Edges) Name(edge Edge) string {
	if edge.Orientation == EdgeDown {
		return e.dots.Name(edge.Dot) + downSuffix
	}
	return e.dots.Name(edge.Dot) + acrossSuffix
}

// All lists every edge of the board, dot by dot from the top left, across before down.
func (e Edges) All() []Edge {
	all := make([]Edge, 0, (e.rows+1)*e.cols+e.rows*(e.cols+1))
	for row := range e.rows + 1 {
		for col := range e.cols + 1 {
			dot := Point{Row: row, Col: col}
			if col < e.cols {
				all = append(all, Edge{Dot: dot, Orientation: EdgeAcross})
			}
			if row < e.rows {
				all = append(all, Edge{Dot: dot, Orientation: EdgeDown})
			}
		}
	}
	return all
}

// Contains reports whether edge lies on the board.
func (e Edges) Contains(edge Edge) bool {
	switch edge.Orientation {
	case EdgeAcross:
		return edge.Dot.Within(e.rows+1, e.cols)
	case EdgeDown:
		return edge.Dot.Within(e.rows, e.cols+1)
	}
	return false
}

// Bordering lists the cells edge is a side of: the one above then below it lying across, the one
// left then right of it lying down, leaving out any off the board.
func (e Edges) Bordering(edge Edge) []Point {
	before := Point{Row: edge.Dot.Row - 1, Col: edge.Dot.Col}
	if edge.Orientation == EdgeDown {
		before = Point{Row: edge.Dot.Row, Col: edge.Dot.Col - 1}
	}
	var cells []Point
	for _, cell := range []Point{before, edge.Dot} {
		if cell.Within(e.rows, e.cols) {
			cells = append(cells, cell)
		}
	}
	return cells
}

// Sides lists cell's four edges: top, bottom, left, right.
func Sides(cell Point) []Edge {
	return []Edge{
		{Dot: cell, Orientation: EdgeAcross},
		{Dot: Point{Row: cell.Row + 1, Col: cell.Col}, Orientation: EdgeAcross},
		{Dot: cell, Orientation: EdgeDown},
		{Dot: Point{Row: cell.Row, Col: cell.Col + 1}, Orientation: EdgeDown},
	}
}
