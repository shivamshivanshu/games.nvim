package gomoku

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/discs"
)

// TypeGomoku is the game's catalogue id.
const TypeGomoku game.GameType = "gomoku"

// View holds Cells[row][column], each the seat whose stone fills it or "" when
// empty. LegalCells is the viewer's own: the empty cells, none unless it is to move.
type View struct {
	Cells        [][]game.Seat  `json:"cells"`
	Last         *discs.Cell    `json:"last" jsonschema:"nullable"`
	WinningLines [][]discs.Cell `json:"winning_lines"`
	LegalCells   []discs.Cell   `json:"legal_cells"`
}

// PlacedEvent reports a stone placed on Cell.
type PlacedEvent struct {
	By   game.Seat  `json:"by"`
	Cell discs.Cell `json:"cell"`
}

// EventPlaced is PlacedEvent's kind.
const EventPlaced game.EventKind = "placed"

// EventKind implements game.Event.
func (PlacedEvent) EventKind() game.EventKind { return EventPlaced }

// ReasonFiveInARow ends a game won by a line of five or more; ReasonBoardFull
// one drawn on a full board.
const (
	ReasonFiveInARow game.ResultReason = "five_in_a_row"
	ReasonBoardFull  game.ResultReason = "board_full"
)
