package tictactoe

import "github.com/shivamshivanshu/games.nvim/server/game"

// TypeTicTacToe is the game's catalogue id.
const TypeTicTacToe game.GameType = "tictactoe"

// SeatX moves first; SeatO second.
const (
	SeatX  game.Seat = "x"
	SeatO  game.Seat = "o"
	noSeat game.Seat = ""
)

var sides = game.Pair{SeatX, SeatO}

// Opts are the resolved challenge options.
type Opts struct {
	First game.FirstMover `json:"first"`
}

// ActionPlace puts the mover's mark on an empty cell.
const ActionPlace game.ActionKind = "place"

// Cell numbers the board in phone layout: 1 at the top-left, 9 at the bottom-right.
type Cell int

// Action is a player's move. Cell is the cell's number as text, "1" to "9".
type Action struct {
	Kind game.ActionKind `json:"kind"`
	Cell string          `json:"cell"`
}

// View is the board as every seat sees it. Cells holds each cell's owner, ""
// when empty. LegalCells is the viewer's own: empty unless it is to move.
type View struct {
	Cells        [cellCount]game.Seat `json:"cells"`
	Last         *Cell                `json:"last" jsonschema:"nullable"`
	WinningLines [][]Cell             `json:"winning_lines"`
	LegalCells   []Cell               `json:"legal_cells"`
}

// PlacedEvent reports a mark placed on Cell.
type PlacedEvent struct {
	By   game.Seat `json:"by"`
	Cell Cell      `json:"cell"`
}

// EventPlaced is PlacedEvent's kind.
const EventPlaced game.EventKind = "placed"

// EventKind implements game.Event.
func (PlacedEvent) EventKind() game.EventKind { return EventPlaced }

// ReasonThreeInARow ends a game won by a line; ReasonBoardFull one drawn on a full board.
const (
	ReasonThreeInARow game.ResultReason = "three_in_a_row"
	ReasonBoardFull   game.ResultReason = "board_full"
)
