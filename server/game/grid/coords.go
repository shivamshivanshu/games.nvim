package grid

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Coords names the cells of a board in letter-number notation: the column's
// letter from a at the left, then the row's number from 1 at the top, as in h8.
type Coords struct {
	rows, cols int
	cell       *regexp.Regexp
}

const (
	maxCoordRows = 19
	maxCoordCols = 26
)

// NewCoords names the cells of a board of rows by cols, which must be 1-19 by 1-26.
func NewCoords(rows, cols int) Coords {
	if rows < 1 || rows > maxCoordRows || cols < 1 || cols > maxCoordCols {
		panic(fmt.Sprintf("grid: no coordinates for %dx%d, want 1-%d rows and 1-%d columns", rows, cols, maxCoordRows, maxCoordCols))
	}
	last := 'a' + rune(cols-1)
	column := fmt.Sprintf("[a-%cA-%c]", last, last-'a'+'A')
	return Coords{rows: rows, cols: cols, cell: regexp.MustCompile(`^\s*(` + column + `)(` + rowAlternatives(rows) + `)\s*$`)}
}

func rowAlternatives(rows int) string {
	if rows <= 9 {
		return fmt.Sprintf("[1-%d]", rows)
	}
	return fmt.Sprintf("1[0-%d]|[1-9]", rows-10)
}

// Pattern is the RE2 expression of the text Parse accepts: a column letter in
// either case, then a row number without leading zeros, with space around it.
func (c Coords) Pattern() string { return c.cell.String() }

// Parse reads the cell text names, reporting false for text Pattern does not match.
func (c Coords) Parse(text string) (Point, bool) {
	match := c.cell.FindStringSubmatch(text)
	if match == nil {
		return Point{}, false
	}
	row, err := strconv.Atoi(match[2])
	if err != nil {
		panic("grid: the row pattern let through a row that is not a number: " + match[2])
	}
	return Point{Row: row - 1, Col: int(strings.ToLower(match[1])[0] - 'a')}, true
}

// Name is the cell's text, in lower case: Parse(Name(p)) is p.
func (c Coords) Name(p Point) string {
	return string(rune('a'+p.Col)) + strconv.Itoa(p.Row+1)
}

// Range is the board's first and last cell, as "a1 to o15".
func (c Coords) Range() string {
	return c.Name(Point{}) + " to " + c.Name(Point{Row: c.rows - 1, Col: c.cols - 1})
}
