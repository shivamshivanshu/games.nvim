package sudoku

import (
	"fmt"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type cellSet struct {
	row, col int
	digit    Digit
}

// parseSet reads "RC D": a row and a column from 1 to 9, then a digit, where 0 clears.
func parseSet(text string) (cellSet, error) {
	fields := strings.Fields(text)
	if len(fields) != 2 || len(fields[0]) != 2 || len(fields[1]) != 1 {
		return cellSet{}, badSet(text)
	}
	row, rowOK := digitAt(fields[0], 0)
	col, colOK := digitAt(fields[0], 1)
	d, digitOK := digitAt(fields[1], 0)
	if !rowOK || !colOK || !digitOK || row == Empty || col == Empty {
		return cellSet{}, badSet(text)
	}
	return cellSet{row: int(row), col: int(col), digit: d}, nil
}

func digitAt(s string, i int) (Digit, bool) {
	if s[i] < '0' || s[i] > '9' {
		return Empty, false
	}
	return Digit(s[i] - '0'), true
}

func badSet(text string) error {
	return game.IllegalActionError{Msg: fmt.Sprintf(`want "RC D", e.g. "53 7" for row 5, column 3, digit 7 (0 clears); got %q`, text)}
}
