package fifteen

import (
	"github.com/shivamshivanshu/games.nvim/server/game"

	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// TypeFifteen is the game type on the wire.
const TypeFifteen game.GameType = "fifteen"

// ReasonSolved ends a game whose tiles are in order.
const ReasonSolved game.ResultReason = "solved"

// Size is the board's side: 3x3 holds tiles 1 to 8, 4x4 tiles 1 to 15.
type Size string

// The sizes.
const (
	Size3 Size = "3x3"
	Size4 Size = "4x4"
)

// OptionSize names the size option.
const OptionSize game.OptionName = "size"

// Opts are the 15-puzzle's resolved options.
type Opts struct {
	Size Size `json:"size"`
}

// ActionSlide slides the tile Tile names into the blank; ActionMove slides the tile that
// moves the way Direction names.
const (
	ActionSlide game.ActionKind = "slide"
	ActionMove  game.ActionKind = "move"
)

// Action is typed input: Tile is the tile's number as text, Direction one of the Directions.
type Action struct {
	Kind      game.ActionKind `json:"kind"`
	Tile      string          `json:"tile,omitempty"`
	Direction string          `json:"direction,omitempty"`
}

// Blank is the empty square's value in View.Tiles.
const Blank = 0

// Movable is a tile next to the blank and the way it would move.
type Movable struct {
	Tile      int            `json:"tile"`
	Direction grid.Direction `json:"direction"`
}

// View holds Tiles[row][col], rows from the top, with Blank for the empty square. Moves counts
// the slides so far and is the score. Movable lists the tiles that may slide now, empty once
// the game is over.
type View struct {
	Tiles   [][]int   `json:"tiles"`
	Moves   int       `json:"moves"`
	Movable []Movable `json:"movable"`
}

// SlidEvent reports a tile slid into the blank and the way it moved.
type SlidEvent struct {
	By        game.Seat      `json:"by"`
	Tile      int            `json:"tile"`
	Direction grid.Direction `json:"direction"`
}

// EventSlid is the kind of SlidEvent.
const EventSlid game.EventKind = "slid"

// EventKind implements game.Event.
func (SlidEvent) EventKind() game.EventKind { return EventSlid }
