package chess

import "github.com/shivamshivanshu/games.nvim/server/game"

// TimeControl and its presets alias the shared ones so existing callers keep compiling.
type TimeControl = game.TimeControl

const (
	TimeControlNone     = game.TimeControlNone
	TimeControl3Plus2   = game.TimeControl3Plus2
	TimeControl5Plus0   = game.TimeControl5Plus0
	TimeControl10Plus5  = game.TimeControl10Plus5
	TimeControl15Plus10 = game.TimeControl15Plus10
)

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
	TimeControl TimeControl `json:"time_control"`
	Color       ColorChoice `json:"color"`
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

type View struct {
	FEN        string       `json:"fen"`
	HistorySAN []string     `json:"history_san"`
	LastMove   *MoveSquares `json:"last_move"`
	InCheck    bool         `json:"in_check"`
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
