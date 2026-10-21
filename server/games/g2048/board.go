package g2048

import (
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

const (
	side = 4
	// twoInTen is the chance in ten that a spawned tile is a 2 rather than a 4.
	twoInTen = 9
)

// board holds board[row][col], rows from the top, Empty where no tile is.
type board [side][side]int

// lineToward lists the cells of row or column i, nearest the edge the tiles slide toward first.
func lineToward(d grid.Direction, i int) [side]grid.Point {
	var line [side]grid.Point
	for k := range side {
		switch d {
		case grid.Up:
			line[k] = grid.Point{Row: k, Col: i}
		case grid.Down:
			line[k] = grid.Point{Row: side - 1 - k, Col: i}
		case grid.Left:
			line[k] = grid.Point{Row: i, Col: k}
		case grid.Right:
			line[k] = grid.Point{Row: i, Col: side - 1 - k}
		}
	}
	return line
}

// slid moves every tile toward d and returns the board and the sum of the merged tiles.
func (b board) slid(d grid.Direction) (board, int) {
	var next board
	gained := 0
	for i := range side {
		line := lineToward(d, i)
		var values [side]int
		for k, p := range line {
			values[k] = b[p.Row][p.Col]
		}
		packed, sum := merged(values)
		for k, p := range line {
			next[p.Row][p.Col] = packed[k]
		}
		gained += sum
	}
	return next, gained
}

// merged packs a line's tiles toward index 0, merging equal neighbours nearest that end first,
// each tile at most once.
func merged(line [side]int) ([side]int, int) {
	var out [side]int
	n, gained, pending := 0, 0, Empty
	for _, value := range line {
		switch value {
		case Empty:
		case pending:
			out[n] = 2 * value
			gained += 2 * value
			n++
			pending = Empty
		default:
			if pending != Empty {
				out[n] = pending
				n++
			}
			pending = value
		}
	}
	if pending != Empty {
		out[n] = pending
	}
	return out, gained
}

// movable lists the directions that would change the board, in the order of grid.Directions().
func (b board) movable() []grid.Direction {
	return slices.DeleteFunc(grid.Directions(), func(d grid.Direction) bool {
		next, _ := b.slid(d)
		return next == b
	})
}

func (b board) contains(value int) bool {
	for _, row := range b {
		if slices.Contains(row[:], value) {
			return true
		}
	}
	return false
}

func (b board) empties() []grid.Point {
	var out []grid.Point
	for row := range side {
		for col := range side {
			if b[row][col] == Empty {
				out = append(out, grid.Point{Row: row, Col: col})
			}
		}
	}
	return out
}

// spawned puts a 2, or a 4 one time in ten, on an empty cell drawn from rng: the cell is drawn
// first, then the value. Every legal move leaves an empty cell, so one always exists.
func (b board) spawned(rng *rand.Rand) (board, Tile) {
	empties := b.empties()
	if len(empties) == 0 {
		panic("g2048: spawn on a full board")
	}
	at := empties[rng.IntN(len(empties))]
	value := 4
	if rng.IntN(10) < twoInTen {
		value = 2
	}
	b[at.Row][at.Col] = value
	return b, Tile{Row: at.Row + 1, Col: at.Col + 1, Value: value}
}

func (b board) cells() [][]int {
	out := make([][]int, side)
	for row := range b {
		out[row] = slices.Clone(b[row][:])
	}
	return out
}
