package checkers

import "github.com/shivamshivanshu/games.nvim/server/game"

// ColorChoice is the challenger's colour. Black moves first.
type ColorChoice string

// Colour choices for the challenger.
const (
	ColorRandom ColorChoice = "random"
	ColorBlack  ColorChoice = "black"
	ColorWhite  ColorChoice = "white"
)

func (c ColorChoice) firstMover() game.FirstMover {
	switch c {
	case ColorBlack:
		return game.FirstMoverMe
	case ColorWhite:
		return game.FirstMoverOpponent
	}
	return game.FirstMoverRandom
}

// Opts are the resolved game options.
type Opts struct {
	TimeControl game.TimeControl `json:"time_control"`
	Color       ColorChoice      `json:"color"`
}

// ActionKind names what an Action does.
type ActionKind string

// ActionMove plays one complete move given as Action.Text.
const ActionMove ActionKind = "move"

// Action is one complete move, such as 11-15 or 15x24x31.
type Action struct {
	Kind ActionKind `json:"kind"`
	Text string     `json:"text,omitempty"`
}

// View is the public position, the same for both seats.
type View struct {
	Board Board `json:"board"`
	// Legal lists every legal move of the side to move, each as its squares from origin to final landing.
	Legal   [][]Square `json:"legal"`
	Last    []Square   `json:"last"`
	History []string   `json:"history"`
}

// MovedEvent reports a played move and whether it crowned a man.
type MovedEvent struct {
	By      game.Seat `json:"by"`
	Move    string    `json:"move"`
	Crowned bool      `json:"crowned"`
}

// EventMoved is the kind of MovedEvent.
const EventMoved game.EventKind = "moved"

// EventKind implements game.Event.
func (MovedEvent) EventKind() game.EventKind { return EventMoved }

// Seats are named by colour.
const (
	SeatBlack game.Seat = "black"
	SeatWhite game.Seat = "white"
)

func opponent(seat game.Seat) game.Seat {
	if seat == SeatBlack {
		return SeatWhite
	}
	return SeatBlack
}

// Reasons a game of checkers ends.
const (
	ReasonNoPieces  game.ResultReason = "no_pieces"
	ReasonNoMoves   game.ResultReason = "no_moves"
	ReasonFortyMove game.ResultReason = "forty_move"
)
