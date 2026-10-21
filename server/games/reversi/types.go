package reversi

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/discs"
)

// TypeReversi is the game's catalogue id.
const TypeReversi game.GameType = "reversi"

// View holds Cells[row][column], each the seat whose disc fills it or "" when
// empty. LegalCells is the viewer's own: the cells it may play, none unless it
// is to move.
type View struct {
	Cells      [][]game.Seat `json:"cells"`
	Last       *discs.Cell   `json:"last" jsonschema:"nullable"`
	LegalCells []discs.Cell  `json:"legal_cells"`
	Discs      DiscCount     `json:"discs"`
}

// DiscCount is how many discs each seat has on the board.
type DiscCount struct {
	Black int `json:"black"`
	White int `json:"white"`
}

// PlacedEvent reports a disc placed on Cell and the opponent's discs it turned over.
type PlacedEvent struct {
	By      game.Seat    `json:"by"`
	Cell    discs.Cell   `json:"cell"`
	Flipped []discs.Cell `json:"flipped"`
}

// PassedEvent reports that By had no legal move, so the turn went back to its opponent.
type PassedEvent struct {
	By game.Seat `json:"by"`
}

// The kinds of PlacedEvent and PassedEvent.
const (
	EventPlaced game.EventKind = "placed"
	EventPassed game.EventKind = "passed"
)

// EventKind implements game.Event.
func (PlacedEvent) EventKind() game.EventKind { return EventPlaced }

// EventKind implements game.Event.
func (PassedEvent) EventKind() game.EventKind { return EventPassed }

// ReasonMoreDiscs ends a game neither seat can move in, won by the seat with
// more discs; ReasonEqualDiscs one drawn on equal counts.
const (
	ReasonMoreDiscs  game.ResultReason = "more_discs"
	ReasonEqualDiscs game.ResultReason = "equal_discs"
)
