package chess

import "games.nvim/server/game"

type TimeControl string

const (
	TimeControlNone     TimeControl = "none"
	TimeControl3Plus2   TimeControl = "3+2"
	TimeControl5Plus0   TimeControl = "5+0"
	TimeControl10Plus5  TimeControl = "10+5"
	TimeControl15Plus10 TimeControl = "15+10"
)

type ColorChoice string

const (
	ColorRandom ColorChoice = "random"
	ColorWhite  ColorChoice = "white"
	ColorBlack  ColorChoice = "black"
)

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

func (MovedEvent) EventKind() string { return "moved" }

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
