package discs

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

// Board is a square board of discs. It is immutable: With returns a changed copy.
type Board struct {
	size   int
	coords grid.Coords
	discs  []game.Seat
}

// NewBoard is an empty board size cells on a side.
func NewBoard(size int) Board {
	return Board{size: size, coords: grid.NewCoords(size, size), discs: make([]game.Seat, size*size)}
}

// Coords names the board's cells.
func (b Board) Coords() grid.Coords { return b.coords }

// Contains reports whether p is on the board.
func (b Board) Contains(p grid.Point) bool { return p.Within(b.size, b.size) }

// At is the disc on p, NoDisc when it is empty.
func (b Board) At(p grid.Point) game.Seat { return b.discs[b.index(p)] }

// With is a copy of the board with seat's disc on every point.
func (b Board) With(seat game.Seat, points ...grid.Point) Board {
	next := Board{size: b.size, coords: b.coords, discs: slices.Clone(b.discs)}
	for _, p := range points {
		next.discs[next.index(p)] = seat
	}
	return next
}

// Rows is a fresh copy of the board, cells[row][col] from the top left.
func (b Board) Rows() [][]game.Seat { return grid.RowsOf(b.discs, b.size) }

// Empty lists the empty points, row by row from the top left.
func (b Board) Empty() []grid.Point {
	points := []grid.Point{}
	for i, disc := range b.discs {
		if disc == NoDisc {
			points = append(points, grid.Point{Row: i / b.size, Col: i % b.size})
		}
	}
	return points
}

// Count is how many of seat's discs are on the board.
func (b Board) Count(seat game.Seat) int { return xslices.Count(b.discs, seat) }

// Full reports whether no cell is empty.
func (b Board) Full() bool { return !slices.Contains(b.discs, NoDisc) }

func (b Board) index(p grid.Point) int { return p.Row*b.size + p.Col }
