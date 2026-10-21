package dotsandboxes

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// TypeDotsAndBoxes is the game's catalogue id.
const TypeDotsAndBoxes game.GameType = "dots_and_boxes"

// Size is the board's side in boxes.
type Size string

// The sizes.
const (
	Size3 Size = "3"
	Size5 Size = "5"
	Size7 Size = "7"
)

// OptionSize names the size option.
const OptionSize game.OptionName = "size"

// Opts are the options a game is created with.
type Opts struct {
	Size        Size             `json:"size"`
	First       game.FirstMover  `json:"first"`
	TimeControl game.TimeControl `json:"time_control"`
}

// ActionDraw draws the edge Edge names.
const ActionDraw game.ActionKind = "draw"

// Action names its edge as text, such as "b2h": the dot at its left or top end, then h lying
// across or v lying down.
type Action struct {
	Kind game.ActionKind `json:"kind"`
	Edge string          `json:"edge"`
}

// Line is an edge on the wire: Row and Col are its left or top dot, 1-based from the top left.
type Line struct {
	Row         int                  `json:"row"`
	Col         int                  `json:"col"`
	Orientation grid.EdgeOrientation `json:"orientation"`
}

// Box is a box on the wire, 1-based: row 1 is the top, column 1 the left.
type Box struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// View holds the board as rows from the top: Across[r][c] is the seat that drew the edge across
// from dot (r, c), Down[r][c] the one down from it, and Boxes[r][c] the seat that completed that
// box, each "" while undrawn or open. Across is a row of dots longer than Boxes, Down a column
// wider. Scores counts each seat's boxes. Last is the latest edge drawn. LegalEdges is the
// viewer's own: the undrawn edges, empty unless it is to move.
type View struct {
	Across     [][]game.Seat     `json:"across"`
	Down       [][]game.Seat     `json:"down"`
	Boxes      [][]game.Seat     `json:"boxes"`
	Scores     map[game.Seat]int `json:"scores"`
	Last       *Line             `json:"last" jsonschema:"nullable"`
	LegalEdges []Line            `json:"legal_edges"`
}

// DrawnEvent reports By's edge and the boxes it completed. Again is true when By moves again,
// having completed a box with the game still on.
type DrawnEvent struct {
	By    game.Seat `json:"by"`
	Edge  Line      `json:"edge"`
	Boxes []Box     `json:"boxes"`
	Again bool      `json:"again"`
}

// EventDrawn is DrawnEvent's kind.
const EventDrawn game.EventKind = "drawn"

// EventKind implements game.Event.
func (DrawnEvent) EventKind() game.EventKind { return EventDrawn }

// ReasonMoreBoxes ends a game won by holding more boxes once every box is complete;
// ReasonEqualBoxes one drawn on equal boxes, which no offered size allows.
const (
	ReasonMoreBoxes  game.ResultReason = "more_boxes"
	ReasonEqualBoxes game.ResultReason = "equal_boxes"
)
