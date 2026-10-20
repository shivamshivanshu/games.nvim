package tictactoe

import "github.com/shivamshivanshu/games.nvim/server/game"

const TypeTicTacToe game.GameType = "tictactoe"

const (
	SeatX  game.Seat = "x"
	SeatO  game.Seat = "o"
	noSeat game.Seat = ""
)

type Opts struct {
	First game.FirstMover `json:"first"`
}

type ActionKind string

const ActionPlace ActionKind = "place"

// Cell numbers the board in phone layout: 1 at the top-left, 9 at the bottom-right.
type Cell int

type Action struct {
	Kind ActionKind `json:"kind"`
	Cell Cell       `json:"cell"`
}

// View.Cells holds each cell's owner, with "" for an empty cell.
type View struct {
	Cells        [cellCount]game.Seat `json:"cells"`
	Last         *Cell                `json:"last"`
	WinningLines [][]Cell             `json:"winning_lines"`
}

type PlacedEvent struct {
	By   game.Seat `json:"by"`
	Cell Cell      `json:"cell"`
}

const EventPlaced game.EventKind = "placed"

func (PlacedEvent) EventKind() game.EventKind { return EventPlaced }

const (
	ReasonThreeInARow game.ResultReason = "three_in_a_row"
	ReasonBoardFull   game.ResultReason = "board_full"
)
