package sudoku

import (
	"regexp"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// setPattern is the catalogue's pattern for ActionSet's text: a row and a column
// from 1 to 9, then a digit, where 0 clears.
const setPattern = `^\s*([1-9])([1-9])\s+([0-9])\s*$`

var setTextPattern = regexp.MustCompile(setPattern)

type cellSet struct {
	row, col int
	digit    Digit
}

func parseSet(text string) (cellSet, error) {
	match := setTextPattern.FindStringSubmatch(text)
	if match == nil {
		return cellSet{}, game.Illegalf(`want "RC D", e.g. "53 7" for row 5, column 3, digit 7 (0 clears); got %q`, text)
	}
	return cellSet{row: int(match[1][0] - '0'), col: int(match[2][0] - '0'), digit: Digit(match[3][0] - '0')}, nil
}
