package uno

import "github.com/shivamshivanshu/games.nvim/server/game"

// The event kinds. Every event's By is the seat whose step caused it; Seat,
// where there is one, is the seat it happened to.
const (
	EventStarted    game.EventKind = "started"
	EventPlayed     game.EventKind = "played"
	EventDrew       game.EventKind = "drew"
	EventPassed     game.EventKind = "passed"
	EventSkipped    game.EventKind = "skipped"
	EventReversed   game.EventKind = "reversed"
	EventColor      game.EventKind = "color"
	EventChallenged game.EventKind = "challenged"
	EventReshuffled game.EventKind = "reshuffled"
	EventLeft       game.EventKind = "left"
)

// StartedEvent is the deal: Top is the card turned up to start the discard pile.
type StartedEvent struct {
	By     game.Seat `json:"by"`
	Dealer game.Seat `json:"dealer"`
	Top    Card      `json:"top"`
}

// PlayedEvent is By playing Card; Color is the colour in play after it.
type PlayedEvent struct {
	By    game.Seat `json:"by"`
	Card  Card      `json:"card"`
	Color Color     `json:"color"`
}

// DrewEvent is Seat drawing Count cards, which only Seat sees.
type DrewEvent struct {
	By    game.Seat `json:"by"`
	Seat  game.Seat `json:"seat"`
	Count int       `json:"count"`
}

// PassedEvent is By keeping the card it drew.
type PassedEvent struct {
	By game.Seat `json:"by"`
}

// SkippedEvent is Seat losing its turn.
type SkippedEvent struct {
	By   game.Seat `json:"by"`
	Seat game.Seat `json:"seat"`
}

// ReversedEvent turns play round to Direction.
type ReversedEvent struct {
	By        game.Seat `json:"by"`
	Direction Direction `json:"direction"`
}

// ColorEvent is By naming the colour for a Wild turned up to start.
type ColorEvent struct {
	By    game.Seat `json:"by"`
	Color Color     `json:"color"`
}

// ChallengedEvent is By challenging Seat's Wild Draw Four. Guilty is whether
// Seat held a card of the colour in play before it, which the server checks
// without showing the hand.
type ChallengedEvent struct {
	By     game.Seat `json:"by"`
	Seat   game.Seat `json:"seat"`
	Guilty bool      `json:"guilty"`
}

// ReshuffledEvent turns the discard pile under its top card into a new draw pile of Cards cards.
type ReshuffledEvent struct {
	By    game.Seat `json:"by"`
	Cards int       `json:"cards"`
}

// LeftEvent is By leaving the game; its hand leaves play.
type LeftEvent struct {
	By game.Seat `json:"by"`
}

// EventKind implements game.Event.
func (StartedEvent) EventKind() game.EventKind { return EventStarted }

// EventKind implements game.Event.
func (PlayedEvent) EventKind() game.EventKind { return EventPlayed }

// EventKind implements game.Event.
func (DrewEvent) EventKind() game.EventKind { return EventDrew }

// EventKind implements game.Event.
func (PassedEvent) EventKind() game.EventKind { return EventPassed }

// EventKind implements game.Event.
func (SkippedEvent) EventKind() game.EventKind { return EventSkipped }

// EventKind implements game.Event.
func (ReversedEvent) EventKind() game.EventKind { return EventReversed }

// EventKind implements game.Event.
func (ColorEvent) EventKind() game.EventKind { return EventColor }

// EventKind implements game.Event.
func (ChallengedEvent) EventKind() game.EventKind { return EventChallenged }

// EventKind implements game.Event.
func (ReshuffledEvent) EventKind() game.EventKind { return EventReshuffled }

// EventKind implements game.Event.
func (LeftEvent) EventKind() game.EventKind { return EventLeft }
