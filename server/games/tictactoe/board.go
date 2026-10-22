package tictactoe

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

const (
	side      = 3
	cellCount = side * side
	winLength = 3
)

type board [cellCount]game.Seat

func parseCell(text string) (CellNumber, error) {
	n, ok := game.ParseNumbered(text, cellCount)
	if !ok {
		return 0, game.Illegalf("cell must be 1-%d, got %q", cellCount, text)
	}
	return CellNumber(n), nil
}

func (c CellNumber) point() grid.Point {
	i := int(c) - 1
	return grid.Point{Row: i / side, Col: i % side}
}

func (b board) owner(c CellNumber) game.Seat { return b[c-1] }

func (b board) with(c CellNumber, seat game.Seat) board {
	b[c-1] = seat
	return b
}

func (b board) emptyCells() []CellNumber {
	cells := []CellNumber{}
	for i, owner := range b {
		if owner == noSeat {
			cells = append(cells, CellNumber(i+1))
		}
	}
	return cells
}

func (b board) full() bool { return !slices.Contains(b[:], noSeat) }

func (b board) rows() [][]game.Seat { return grid.RowsOf(b[:], side) }

func (b board) winningLinesThrough(c CellNumber) [][]grid.Cell {
	return grid.CellLines(grid.WinningLines(b.rows(), c.point(), winLength, noSeat))
}
