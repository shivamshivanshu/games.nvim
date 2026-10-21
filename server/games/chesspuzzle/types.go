package chesspuzzle

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
)

// TypeChessPuzzle is the game type on the wire.
const TypeChessPuzzle game.GameType = "chess_puzzle"

// SeatSolver is the only seat.
const SeatSolver game.Seat = "solver"

// Mode is how a move that is not the line's is taken.
type Mode string

// The modes.
const (
	// ModePuzzle ends the game at the first wrong move.
	ModePuzzle Mode = "puzzle"
	// ModeStudy counts a wrong guess as a miss and waits for the right one.
	ModeStudy Mode = "study"
)

// Side is the colour the solver plays; the server plays the other.
type Side string

// The sides.
const (
	SideWhite Side = "white"
	SideBlack Side = "black"
)

// The option names.
const (
	OptionMode   game.OptionName = "mode"
	OptionFEN    game.OptionName = "fen"
	OptionMoves  game.OptionName = "moves"
	OptionPlayer game.OptionName = "player"
)

// Opts are the resolved options. An empty FEN is the standard start position;
// Moves is the whole line, both sides, space-separated SAN or UCI.
type Opts struct {
	Mode   Mode   `json:"mode"`
	FEN    string `json:"fen"`
	Moves  string `json:"moves"`
	Player Side   `json:"player"`
}

// The action kinds.
const (
	// ActionMove plays Text as the solver's move.
	ActionMove game.ActionKind = "move"
	// ActionReveal plays the line's move for the solver, as a miss; study mode only.
	ActionReveal game.ActionKind = "reveal"
)

// Action is typed input, sent raw and parsed by the server.
type Action struct {
	Kind game.ActionKind `json:"kind"`
	Text string          `json:"text,omitempty"`
}

// View is the chess position as chess shows it, plus the puzzle's progress.
// LegalMoves (UCI) is empty once the game is over; Remaining counts the
// solver's moves left in the line; Solution is the whole line in SAN, empty
// until the game is over.
type View struct {
	FEN         string             `json:"fen"`
	HistorySAN  []string           `json:"history_san"`
	LastMove    *chess.MoveSquares `json:"last_move" jsonschema:"nullable"`
	LegalMoves  []string           `json:"legal_moves"`
	CheckSquare *string            `json:"check_square" jsonschema:"nullable"`
	Mode        Mode               `json:"mode"`
	Misses      int                `json:"misses"`
	Remaining   int                `json:"remaining"`
	Solution    []string           `json:"solution"`
}

// The ways a game ends other than giving up.
const (
	ReasonSolved    game.ResultReason = "solved"
	ReasonWrongMove game.ResultReason = "wrong_move"
	ReasonCompleted game.ResultReason = "completed"
)

// MovedEvent reports the solver's move that the line accepted.
type MovedEvent struct {
	By  game.Seat `json:"by"`
	SAN string    `json:"san"`
}

// RepliedEvent reports a line move the server played for the other side.
type RepliedEvent struct {
	By  game.Seat `json:"by"`
	SAN string    `json:"san"`
}

// MissedEvent reports a legal move that is not the line's: in study mode the
// position stays, in puzzle mode the game ends.
type MissedEvent struct {
	By  game.Seat `json:"by"`
	SAN string    `json:"san"`
}

// RevealedEvent reports the line's move, played for the solver as a miss.
type RevealedEvent struct {
	By  game.Seat `json:"by"`
	SAN string    `json:"san"`
}

// The event kinds.
const (
	EventMoved    game.EventKind = "moved"
	EventReplied  game.EventKind = "replied"
	EventMissed   game.EventKind = "missed"
	EventRevealed game.EventKind = "revealed"
)

// EventKind implements game.Event.
func (MovedEvent) EventKind() game.EventKind { return EventMoved }

// EventKind implements game.Event.
func (RepliedEvent) EventKind() game.EventKind { return EventReplied }

// EventKind implements game.Event.
func (MissedEvent) EventKind() game.EventKind { return EventMissed }

// EventKind implements game.Event.
func (RevealedEvent) EventKind() game.EventKind { return EventRevealed }
