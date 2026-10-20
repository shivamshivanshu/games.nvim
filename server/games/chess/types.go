package chess

import "github.com/shivamshivanshu/games.nvim/server/game"

type ColorChoice string

const (
	ColorRandom ColorChoice = "random"
	ColorWhite  ColorChoice = "white"
	ColorBlack  ColorChoice = "black"
)

func (c ColorChoice) firstMover() game.FirstMover {
	switch c {
	case ColorWhite:
		return game.FirstMoverMe
	case ColorBlack:
		return game.FirstMoverOpponent
	}
	return game.FirstMoverRandom
}

type Opts struct {
	TimeControl game.TimeControl `json:"time_control"`
	Color       ColorChoice      `json:"color"`
}

type ActionKind string

const (
	ActionMove      ActionKind = "move"
	ActionClaimDraw ActionKind = "claim_draw"
)

type Action struct {
	Kind ActionKind `json:"kind"`
	Move string     `json:"move,omitempty"`
}

type MoveSquares struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// View.LegalMoves and View.ClaimableDraw are the viewer's own: empty and
// false for a seat that is not to move. CheckSquare holds the king of the
// side to move while it is in check.
type View struct {
	FEN           string       `json:"fen"`
	HistorySAN    []string     `json:"history_san"`
	LastMove      *MoveSquares `json:"last_move"`
	LegalMoves    []string     `json:"legal_moves"`
	ClaimableDraw bool         `json:"claimable_draw"`
	CheckSquare   *string      `json:"check_square"`
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
