package poker

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
)

// Event kinds. Every event's By is the seat whose step caused it, so a deal,
// blind or award that follows a move is by that move's author.
const (
	EventHandStarted game.EventKind = "hand_started"
	EventPosted      game.EventKind = "posted"
	EventActed       game.EventKind = "acted"
	EventBoard       game.EventKind = "board"
	EventShown       game.EventKind = "shown"
	EventReturned    game.EventKind = "returned"
	EventWon         game.EventKind = "won"
	EventBusted      game.EventKind = "busted"
	EventLeft        game.EventKind = "left"
)

// HandStartedEvent deals a new hand.
type HandStartedEvent struct {
	By     game.Seat `json:"by"`
	HandNo int       `json:"hand_no"`
	Button game.Seat `json:"button"`
	Blinds Blinds    `json:"blinds"`
}

// PostedEvent is a blind posted by Seat; Amount is less than the blind when it put Seat all-in.
type PostedEvent struct {
	By     game.Seat `json:"by"`
	Seat   game.Seat `json:"seat"`
	Blind  BlindKind `json:"blind"`
	Amount Chips     `json:"amount"`
}

// ActedEvent is a betting move. Amount is By's total for the street after it, 0 for fold and check.
type ActedEvent struct {
	By     game.Seat `json:"by"`
	Move   MoveKind  `json:"move"`
	Amount Chips     `json:"amount"`
}

// BoardEvent deals a street's board cards.
type BoardEvent struct {
	By     game.Seat    `json:"by"`
	Street Street       `json:"street"`
	Cards  []cards.Card `json:"cards"`
}

// ShownEvent reveals every hand still live once no more betting is possible.
type ShownEvent struct {
	By    game.Seat   `json:"by"`
	Hands []ShownHand `json:"hands"`
}

// ReturnedEvent gives Seat back the part of its bet that no one called.
type ReturnedEvent struct {
	By     game.Seat `json:"by"`
	Seat   game.Seat `json:"seat"`
	Amount Chips     `json:"amount"`
}

// WonEvent awards Seat a pot or its share of one. Pot 0 is the main pot.
type WonEvent struct {
	By     game.Seat `json:"by"`
	Seat   game.Seat `json:"seat"`
	Amount Chips     `json:"amount"`
	Pot    int       `json:"pot"`
}

// BustedEvent is Seat losing its last chip, which puts it out of the game.
type BustedEvent struct {
	By   game.Seat `json:"by"`
	Seat game.Seat `json:"seat"`
}

// LeftEvent is By leaving the game; its stack leaves play.
type LeftEvent struct {
	By game.Seat `json:"by"`
}

// EventKind implements game.Event.
func (HandStartedEvent) EventKind() game.EventKind { return EventHandStarted }

// EventKind implements game.Event.
func (PostedEvent) EventKind() game.EventKind { return EventPosted }

// EventKind implements game.Event.
func (ActedEvent) EventKind() game.EventKind { return EventActed }

// EventKind implements game.Event.
func (BoardEvent) EventKind() game.EventKind { return EventBoard }

// EventKind implements game.Event.
func (ShownEvent) EventKind() game.EventKind { return EventShown }

// EventKind implements game.Event.
func (ReturnedEvent) EventKind() game.EventKind { return EventReturned }

// EventKind implements game.Event.
func (WonEvent) EventKind() game.EventKind { return EventWon }

// EventKind implements game.Event.
func (BustedEvent) EventKind() game.EventKind { return EventBusted }

// EventKind implements game.Event.
func (LeftEvent) EventKind() game.EventKind { return EventLeft }
