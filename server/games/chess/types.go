package chess

import (
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type ColorChoice string

const (
	ColorRandom ColorChoice = "random"
	ColorWhite  ColorChoice = "white"
	ColorBlack  ColorChoice = "black"
)

type Opts struct {
	TimeControl game.TimeControl `json:"time_control"`
	Color       ColorChoice      `json:"color"`
}

const (
	ActionMove      game.ActionKind = "move"
	ActionClaimDraw game.ActionKind = "claim_draw"
)

type Action struct {
	Kind game.ActionKind `json:"kind"`
	Move string          `json:"move,omitempty"`
}

type MoveSquares struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// PieceLetter is a piece as FEN writes it: uppercase for white, lowercase for black.
type PieceLetter string

// EnumValues lists the white pieces, then the black.
func (PieceLetter) EnumValues() []string {
	white := pieceLetters[NoPieceKind+1:]
	return strings.Split(white+strings.ToLower(white), "")
}

// SideMaterial is one side's material lead: Pieces are the opponent's pieces
// it is up, most valuable first, and Score the points it is ahead, 0 unless
// it is ahead.
type SideMaterial struct {
	Pieces []PieceLetter `json:"pieces"`
	Score  int           `json:"score"`
}

// Material is each side's lead on the board.
type Material struct {
	White SideMaterial `json:"white"`
	Black SideMaterial `json:"black"`
}

// View.LegalMoves and View.ClaimableDraw are the viewer's own: empty and
// false for a seat that is not to move. CheckSquare holds the king of the
// side to move while it is in check. FirstPly is how many plies the start
// position has behind it, which the move list numbers on from.
type View struct {
	FEN           string       `json:"fen"`
	HistorySAN    []string     `json:"history_san"`
	LastMove      *MoveSquares `json:"last_move" jsonschema:"nullable"`
	LegalMoves    []string     `json:"legal_moves"`
	ClaimableDraw bool         `json:"claimable_draw"`
	CheckSquare   *string      `json:"check_square" jsonschema:"nullable"`
	Material      Material     `json:"material"`
	FirstPly      int          `json:"first_ply"`
}

type MovedEvent struct {
	By  game.Seat `json:"by"`
	SAN string    `json:"san"`
}

const EventMoved game.EventKind = "moved"

func (MovedEvent) EventKind() game.EventKind { return EventMoved }

const (
	SeatWhite game.Seat = "white"
	SeatBlack game.Seat = "black"
)

var sides = game.Pair{SeatWhite, SeatBlack}

const (
	ReasonCheckmate            game.ResultReason = "checkmate"
	ReasonStalemate            game.ResultReason = "stalemate"
	ReasonInsufficientMaterial game.ResultReason = "insufficient_material"
	ReasonFivefold             game.ResultReason = "fivefold_repetition"
	ReasonSeventyFive          game.ResultReason = "seventy_five_move"
	ReasonThreefold            game.ResultReason = "threefold_repetition"
	ReasonFifty                game.ResultReason = "fifty_move"
)

func seatOf(c Color) game.Seat {
	if c == White {
		return SeatWhite
	}
	return SeatBlack
}
