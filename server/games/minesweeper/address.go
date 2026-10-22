package minesweeper

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// cellSyntax is the catalogue's pattern for an action's cell: a row letter a to p, either
// case, then a column number 1 to 30. Whether the cell is on the board depends on the difficulty.
var cellSyntax = regexp.MustCompile(`^\s*([a-pA-P])([1-9]|[12][0-9]|30)\s*$`)

const firstRowLetter = 'a'

func (l level) parseCell(text string) (int, error) {
	match := cellSyntax.FindStringSubmatch(text)
	if match == nil {
		return 0, game.Illegalf(`want a row letter then a column number, e.g. "c12"; got %q`, text)
	}
	col, err := strconv.Atoi(match[2])
	if err != nil {
		return 0, fmt.Errorf("column %q: %w", match[2], err)
	}
	p := grid.Point{Row: int(strings.ToLower(match[1])[0] - firstRowLetter), Col: col - 1}
	if p.Row >= l.rows || p.Col >= l.cols {
		return 0, game.Illegalf("no cell %s on a board of %d rows by %d columns", address(p), l.rows, l.cols)
	}
	return l.index(p), nil
}

func address(p grid.Point) string {
	return fmt.Sprintf("%c%d", firstRowLetter+rune(p.Row), p.Col+1)
}
