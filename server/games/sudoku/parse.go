package sudoku

import (
	"regexp"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

var (
	// setSyntax is the catalogue's pattern for ActionSet's text: a row and a column
	// from 1 to 9, then a digit, where 0 clears.
	setSyntax = regexp.MustCompile(`^\s*([1-9])([1-9])\s+([0-9])\s*$`)
	// clearSyntax is the catalogue's pattern for ActionClear's cell: a row and a column from 1 to 9.
	clearSyntax = regexp.MustCompile(`^\s*([1-9])([1-9])\s*$`)
)

type cellSet struct {
	row, col int
	digit    Digit
}

func parseAction(action Action) (cellSet, error) {
	switch action.Kind {
	case ActionSet:
		return parseSet(action.Text)
	case ActionClear:
		return parseClear(action.Cell)
	}
	return cellSet{}, game.UnknownAction(action.Kind)
}

func parseSet(text string) (cellSet, error) {
	match := setSyntax.FindStringSubmatch(text)
	if match == nil {
		return cellSet{}, game.Illegalf(`want "RC D", e.g. "53 7" for row 5, column 3, digit 7 (0 clears); got %q`, text)
	}
	return cellSet{row: int(match[1][0] - '0'), col: int(match[2][0] - '0'), digit: Digit(match[3][0] - '0')}, nil
}

func parseClear(cell string) (cellSet, error) {
	match := clearSyntax.FindStringSubmatch(cell)
	if match == nil {
		return cellSet{}, game.Illegalf(`want "RC", e.g. "53" for row 5, column 3; got %q`, cell)
	}
	return cellSet{row: int(match[1][0] - '0'), col: int(match[2][0] - '0'), digit: Empty}, nil
}
