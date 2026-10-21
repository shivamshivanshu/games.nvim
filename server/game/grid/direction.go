package grid

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// Direction is one of the four orthogonal ways across a board, named as on the wire.
type Direction string

// The directions.
const (
	Up    Direction = "up"
	Down  Direction = "down"
	Left  Direction = "left"
	Right Direction = "right"
)

var directions = [...]Direction{Up, Down, Left, Right}

// Directions lists the directions in catalogue order: up, down, left, right.
func Directions() []Direction { return slices.Clone(directions[:]) }

// DirectionNames lists the text ParseDirection accepts, in the order of Directions, for an
// action catalogue's Values.
func DirectionNames() []string { return game.Strings(directions[:]...) }

func (Direction) EnumValues() []string { return DirectionNames() }

// ParseDirection reads a direction's name, failing with a game.IllegalActionError for any
// text not in DirectionNames.
func ParseDirection(text string) (Direction, error) {
	for _, d := range directions {
		if text == string(d) {
			return d, nil
		}
	}
	return "", game.Illegalf("want up, down, left or right; got %q", text)
}

// From is the point one cell from p toward d, row 0 at the top; it may lie off the board.
func (d Direction) From(p Point) Point { return p.step(d.delta(), 1) }

// Delta is the change in row and column of one cell toward d, row 0 at the top.
func (d Direction) Delta() (dRow, dCol int) {
	step := d.delta()
	return step.dRow, step.dCol
}

func (d Direction) delta() delta {
	switch d {
	case Up:
		return delta{-1, 0}
	case Down:
		return delta{1, 0}
	case Left:
		return delta{0, -1}
	}
	return delta{0, 1}
}

// Opposite is the way back: up for down, left for right.
func (d Direction) Opposite() Direction {
	switch d {
	case Up:
		return Down
	case Down:
		return Up
	case Left:
		return Right
	}
	return Left
}
