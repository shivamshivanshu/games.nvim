package connect4

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// Opts are the options a connect4 game is created with.
type Opts struct {
	First       game.FirstMover  `json:"first"`
	TimeControl game.TimeControl `json:"time_control"`
}

// ActionDrop drops the player's disc into a column.
const ActionDrop game.ActionKind = "drop"

const columnField = "column"

// Action is a drop into Column, the column's number as text: "1" to "7" from the left.
type Action struct {
	Kind   game.ActionKind `json:"kind"`
	Column string          `json:"column"`
}

// View holds Cells[row][column], each the seat whose disc fills it or "" when empty.
// LegalColumns is the viewer's own: the columns it may drop into, empty unless it is to move.
type View struct {
	Cells        [][]game.Seat `json:"cells"`
	Last         *grid.Cell    `json:"last" jsonschema:"nullable"`
	WinningLines [][]grid.Cell `json:"winning_lines"`
	LegalColumns []int         `json:"legal_columns"`
}

// DroppedEvent says which cell a seat's disc came to rest in.
type DroppedEvent struct {
	By   game.Seat `json:"by"`
	Cell grid.Cell `json:"cell"`
}

// EventDropped is the kind of DroppedEvent.
const EventDropped game.EventKind = "dropped"

// EventKind is EventDropped.
func (DroppedEvent) EventKind() game.EventKind { return EventDropped }

// SeatRed moves first; SeatYellow second.
const (
	SeatRed    game.Seat = "red"
	SeatYellow game.Seat = "yellow"
	noDisc     game.Seat = ""
)

var sides = game.Pair{SeatRed, SeatYellow}

// ReasonFourInARow ends a game won by a line of four or more; ReasonBoardFull
// ends one drawn because no column is left.
const (
	ReasonFourInARow game.ResultReason = "four_in_a_row"
	ReasonBoardFull  game.ResultReason = "board_full"
)
