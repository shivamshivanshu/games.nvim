package fifteen

import (
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// board holds the tiles row-major, Blank for the empty square.
type board struct {
	side  int
	tiles []int
}

func solvedBoard(side int) board {
	tiles := make([]int, side*side)
	for i := range len(tiles) - 1 {
		tiles[i] = i + 1
	}
	return board{side: side, tiles: tiles}
}

// shuffled draws a random arrangement from rng, swaps two tiles if it cannot be solved, and
// draws again if that leaves it solved.
func shuffled(side int, rng *rand.Rand) board {
	for {
		b := board{side: side, tiles: rng.Perm(side * side)}
		if !b.solvable() {
			b = b.swappedTiles()
		}
		if !b.solved() {
			return b
		}
	}
}

// solvable holds when the parity of the tiles' permutation, blank included, equals the parity
// of the blank's distance from its home: every slide flips both.
func (b board) solvable() bool {
	return b.permutationParity() == b.blankDistance()%2
}

func (b board) permutationParity() int {
	home := func(tile int) int {
		if tile == Blank {
			return len(b.tiles) - 1
		}
		return tile - 1
	}
	seen := make([]bool, len(b.tiles))
	cycles := 0
	for start := range b.tiles {
		if seen[start] {
			continue
		}
		cycles++
		for i := start; !seen[i]; i = home(b.tiles[i]) {
			seen[i] = true
		}
	}
	return (len(b.tiles) - cycles) % 2
}

func (b board) blankDistance() int {
	p := b.point(b.blank())
	return (b.side - 1 - p.Row) + (b.side - 1 - p.Col)
}

// swappedTiles exchanges the first two tiles that are not the blank, which flips solvability.
func (b board) swappedTiles() board {
	tiles := slices.Clone(b.tiles)
	first, second := 0, 1
	if tiles[first] == Blank {
		first = 2
	} else if tiles[second] == Blank {
		second = 2
	}
	tiles[first], tiles[second] = tiles[second], tiles[first]
	return board{side: b.side, tiles: tiles}
}

func (b board) solved() bool { return slices.Equal(b.tiles, solvedBoard(b.side).tiles) }

func (b board) blank() int { return slices.Index(b.tiles, Blank) }

func (b board) point(i int) grid.Point { return grid.Point{Row: i / b.side, Col: i % b.side} }

func (b board) index(p grid.Point) int { return p.Row*b.side + p.Col }

// movable lists the tiles beside the blank and the way each would move, in row-major order.
func (b board) movable() []Movable {
	blank := b.point(b.blank())
	neighbours := grid.Neighbours(blank, b.side, b.side, grid.Orthogonal)
	out := make([]Movable, len(neighbours))
	for i, n := range neighbours {
		out[i] = Movable{Tile: b.tiles[b.index(n)], Direction: towards(n, blank)}
	}
	return out
}

func towards(from, to grid.Point) Direction {
	switch {
	case to.Row < from.Row:
		return DirectionUp
	case to.Row > from.Row:
		return DirectionDown
	case to.Col < from.Col:
		return DirectionLeft
	}
	return DirectionRight
}

// slid moves tile into the blank.
func (b board) slid(tile int) board {
	tiles := slices.Clone(b.tiles)
	from, blank := slices.Index(tiles, tile), b.blank()
	tiles[from], tiles[blank] = Blank, tile
	return board{side: b.side, tiles: tiles}
}
