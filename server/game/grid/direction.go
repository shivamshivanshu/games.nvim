package grid

import (
	"fmt"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
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

// Directions lists the directions in catalogue order: up, down, left, right.
func Directions() []Direction { return []Direction{Up, Down, Left, Right} }

// DirectionNames lists the text ParseDirection accepts, in the order of Directions, for an
// action catalogue's Values.
func DirectionNames() []string {
	return xslices.Map(Directions(), func(d Direction) string { return string(d) })
}

// ParseDirection reads a direction's name, failing with a game.IllegalActionError for any
// text not in DirectionNames.
func ParseDirection(text string) (Direction, error) {
	for _, d := range Directions() {
		if text == string(d) {
			return d, nil
		}
	}
	return "", game.IllegalActionError{Msg: fmt.Sprintf("want up, down, left or right; got %q", text)}
}

// From is the point one cell from p toward d, row 0 at the top; it may lie off the board.
func (d Direction) From(p Point) Point {
	switch d {
	case Up:
		return Point{Row: p.Row - 1, Col: p.Col}
	case Down:
		return Point{Row: p.Row + 1, Col: p.Col}
	case Left:
		return Point{Row: p.Row, Col: p.Col - 1}
	}
	return Point{Row: p.Row, Col: p.Col + 1}
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
