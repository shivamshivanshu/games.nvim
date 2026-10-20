package tictactoe

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

const (
	side      = 3
	cellCount = side * side
	winLength = 3
)

type board [cellCount]game.Seat

// cellInputs are the texts a place action accepts, "1" to "9", in cell order.
func cellInputs() []string {
	inputs := make([]string, cellCount)
	for i := range inputs {
		inputs[i] = strconv.Itoa(i + 1)
	}
	return inputs
}

func parseCell(text string) (Cell, error) {
	i := slices.Index(cellInputs(), text)
	if i < 0 {
		return 0, game.IllegalActionError{Msg: fmt.Sprintf("cell must be 1-%d, got %q", cellCount, text)}
	}
	return Cell(i + 1), nil
}

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

func (b board) emptyCells() []Cell {
	cells := []Cell{}
	for i, owner := range b {
		if owner == noSeat {
			cells = append(cells, Cell(i+1))
		}
	}
	return cells
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
