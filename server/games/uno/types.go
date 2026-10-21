package uno

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
)

// TypeUno is the game type on the wire.
const TypeUno game.GameType = "uno"

const (
	minPlayers = 2
	maxPlayers = 6
	handSize   = 7
)

// ReasonWentOut ends the game when a seat plays its last card.
const ReasonWentOut game.ResultReason = "went_out"

// Opts is empty: one hand under the official rules, with no options.
type Opts struct{}

// The action kinds.
const (
	ActionPlay      game.ActionKind = "play"
	ActionDraw      game.ActionKind = "draw"
	ActionPass      game.ActionKind = "pass"
	ActionChallenge game.ActionKind = "challenge"
	ActionAccept    game.ActionKind = "accept"
	ActionColor     game.ActionKind = "color"
)

const (
	cardField  = "card"
	colorField = "color"
)

// Action is a move. Play takes Card, a card from the hand as typed, e.g.
// "r5", or a wild and the colour it names, "w g" or "w+4 b". Color takes
// Color, the colour named for a Wild turned up to start. The others take
// nothing.
type Action struct {
	Kind  game.ActionKind `json:"kind"`
	Card  string          `json:"card,omitempty"`
	Color string          `json:"color,omitempty"`
}

// Phase is what the seat to act must do.
type Phase string

// The phases.
const (
	// PhaseColor is naming the colour for a Wild turned up to start.
	PhaseColor Phase = "color"
	// PhasePlay is playing a card or drawing one.
	PhasePlay Phase = "play"
	// PhaseDrawn is playing the card just drawn, or keeping it.
	PhaseDrawn Phase = "drawn"
	// PhaseChallenge is challenging a Wild Draw Four, or drawing its four cards.
	PhaseChallenge Phase = "challenge"
)

// Direction is the way play goes round the table.
type Direction string

// The directions; seats are numbered clockwise.
const (
	Clockwise        Direction = "clockwise"
	CounterClockwise Direction = "counterclockwise"
)

// View is the table as one seat sees it: its own hand, and every other hand
// only as a count.
//
// Color is the colour in play: the top card's, or the one its wild named.
// Legal and Playable are the viewer's own, empty unless it is to act:
// Legal the action kinds it may take, Playable the cards of its hand that
// play accepts now, a wild listed bare, as it must still name a colour.
type View struct {
	Top       Card              `json:"top"`
	Color     Color             `json:"color"`
	Direction Direction         `json:"direction"`
	Phase     Phase             `json:"phase"`
	DrawPile  int               `json:"draw_pile"`
	Seats     []SeatView        `json:"seats"`
	MyHand    []Card            `json:"my_hand"`
	Legal     []game.ActionKind `json:"legal"`
	Playable  []Card            `json:"playable"`
}

// SeatView is one seat at the table, in seat order. Out is a seat that left
// the game.
type SeatView struct {
	Seat  game.Seat `json:"seat"`
	Cards int       `json:"cards"`
	Out   bool      `json:"out"`
}
