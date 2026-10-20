package connect4

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const (
	boardRows    = 6
	boardColumns = 7
	winLength    = 4
)

type board [boardRows][boardColumns]game.Seat

// landingRow is the 0-based row a disc dropped into the 0-based column comes to rest in.
func (b board) landingRow(column int) (int, bool) {
	for row := boardRows - 1; row >= 0; row-- {
		if b[row][column] == noDisc {
			return row, true
		}
	}
	return 0, false
}

func (b board) full() bool {
	return !slices.Contains(b[0][:], noDisc)
}

func (b board) cells() [][]game.Seat {
	out := make([][]game.Seat, boardRows)
	for row := range b {
		out[row] = slices.Clone(b[row][:])
	}
	return out
}
