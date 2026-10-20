package tictactoe

import (
	"fmt"
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

func (c Cell) point() grid.Point {
	i := int(c) - 1
	return grid.Point{Row: i / side, Col: i % side}
}

func cellAt(p grid.Point) Cell { return Cell(p.Row*side + p.Col + 1) }

func (b board) owner(c Cell) game.Seat { return b[c-1] }

func (b board) with(c Cell, seat game.Seat) board {
	b[c-1] = seat
	return b
}

func (b board) full() bool { return !slices.Contains(b[:], noSeat) }

func (b board) placeError(c Cell) error {
	switch {
	case c < 1 || c > cellCount:
		return game.IllegalActionError{Msg: fmt.Sprintf("cell must be 1-%d, got %d", cellCount, c)}
	case b.owner(c) != noSeat:
		return game.IllegalActionError{Msg: fmt.Sprintf("cell %d is taken", c)}
	}
	return nil
}

func (b board) rows() [][]game.Seat {
	rows := make([][]game.Seat, side)
	for r := range rows {
		rows[r] = b[r*side : (r+1)*side]
	}
	return rows
}

func (b board) winningLinesThrough(c Cell) [][]Cell {
	lines := [][]Cell{}
	for _, points := range grid.WinningLines(b.rows(), c.point(), winLength, noSeat) {
		line := make([]Cell, len(points))
		for i, p := range points {
			line[i] = cellAt(p)
		}
		lines = append(lines, line)
	}
	return lines
}
