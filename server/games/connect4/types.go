package connect4

import "github.com/shivamshivanshu/games.nvim/server/game"

type Opts struct {
	First       game.FirstMover  `json:"first"`
	TimeControl game.TimeControl `json:"time_control"`
}

type ActionKind string

const ActionDrop ActionKind = "drop"

// Action drops a disc into Column, numbered 1 to 7 from the left.
type Action struct {
	Kind   ActionKind `json:"kind"`
	Column int        `json:"column"`
}

// Cell is a 1-based board position; row 1 is the top.
type Cell struct {
	Row    int `json:"row"`
	Column int `json:"column"`
}

// View holds Cells[row][column], each the seat whose disc fills it or "" when empty.
type View struct {
	Cells        [][]game.Seat `json:"cells"`
	Last         *Cell         `json:"last"`
	WinningLines [][]Cell      `json:"winning_lines"`
}

type DroppedEvent struct {
	By     game.Seat `json:"by"`
	Row    int       `json:"row"`
	Column int       `json:"column"`
}

const EventDropped game.EventKind = "dropped"

func (DroppedEvent) EventKind() game.EventKind { return EventDropped }

const (
	SeatRed    game.Seat = "red"
	SeatYellow game.Seat = "yellow"
	noDisc     game.Seat = ""
)

const (
	ReasonFourInARow game.ResultReason = "four_in_a_row"
	ReasonBoardFull  game.ResultReason = "board_full"
)
