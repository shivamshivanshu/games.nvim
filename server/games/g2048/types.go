package g2048

import (
	"github.com/shivamshivanshu/games.nvim/server/game"

	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// TypeG2048 is the game type on the wire.
const TypeG2048 game.GameType = "g2048"

// ReasonNoMoves ends a game whose board no move can change.
const ReasonNoMoves game.ResultReason = "no_moves"

// Goal is the tile whose first appearance marks the game reached; play goes on after it.
const Goal = 2048

// Opts is empty: 2048 has no options.
type Opts struct{}

// ActionMove slides every tile the way Direction names.
const ActionMove game.ActionKind = "move"

// Action is a move; Direction is one of the Directions as text.
type Action struct {
	Kind      game.ActionKind `json:"kind"`
	Direction string          `json:"direction"`
}

// Empty is an empty cell's value in View.Cells.
const Empty = 0

// View holds Cells[row][col], rows from the top, each a tile's value or Empty. Score is the sum
// of every merge's value so far. Reached is true once a Goal tile has been made. Movable lists
// the directions that would change the board, empty once the game is over.
type View struct {
	Cells   [][]int          `json:"cells"`
	Score   int              `json:"score"`
	Reached bool             `json:"reached"`
	Movable []grid.Direction `json:"movable"`
}

// Tile is a 1-based cell, row 1 at the top, and the value it holds.
type Tile struct {
	Row   int `json:"row"`
	Col   int `json:"col"`
	Value int `json:"value"`
}

// MovedEvent reports a move: the score it gained and the tile that appeared after it.
type MovedEvent struct {
	By        game.Seat      `json:"by"`
	Direction grid.Direction `json:"direction"`
	Gained    int            `json:"gained"`
	Spawned   Tile           `json:"spawned"`
}

// EventMoved is the kind of MovedEvent.
const EventMoved game.EventKind = "moved"

// EventKind implements game.Event.
func (MovedEvent) EventKind() game.EventKind { return EventMoved }

// ReachedEvent reports the move that first made a Goal tile.
type ReachedEvent struct {
	By   game.Seat `json:"by"`
	Tile int       `json:"tile"`
}

// EventReached is the kind of ReachedEvent.
const EventReached game.EventKind = "reached"

// EventKind implements game.Event.
func (ReachedEvent) EventKind() game.EventKind { return EventReached }
