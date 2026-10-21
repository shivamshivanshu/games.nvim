package hearts

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
)

// The event kinds. Every event's By is the seat whose step caused it; Seat,
// where there is one, is the seat it happened to, which differs from By for
// a seat that left and is played for.
const (
	EventHandStarted game.EventKind = "hand_started"
	EventPassChosen  game.EventKind = "pass_chosen"
	EventPassed      game.EventKind = "passed"
	EventPlayed      game.EventKind = "played"
	EventTrickWon    game.EventKind = "trick_won"
	EventHandScored  game.EventKind = "hand_scored"
	EventLeft        game.EventKind = "left"
)

// HandStartedEvent deals hand HandNo, whose passes go Pass.
type HandStartedEvent struct {
	By     game.Seat     `json:"by"`
	HandNo int           `json:"hand_no"`
	Pass   PassDirection `json:"pass"`
}

// PassChosenEvent is Seat choosing its three cards to pass, which only it and its receiver see.
type PassChosenEvent struct {
	By   game.Seat `json:"by"`
	Seat game.Seat `json:"seat"`
}

// PassedEvent hands every seat's chosen cards on, Pass.
type PassedEvent struct {
	By   game.Seat     `json:"by"`
	Pass PassDirection `json:"pass"`
}

// PlayedEvent is Seat playing Card to the trick.
type PlayedEvent struct {
	By   game.Seat  `json:"by"`
	Seat game.Seat  `json:"seat"`
	Card cards.Card `json:"card"`
}

// TrickWonEvent is Seat taking the trick, and its Points.
type TrickWonEvent struct {
	By     game.Seat `json:"by"`
	Seat   game.Seat `json:"seat"`
	Points int       `json:"points"`
}

// HandScoredEvent ends a hand with what each seat scored in it.
type HandScoredEvent struct {
	By    game.Seat `json:"by"`
	Score HandScore `json:"score"`
}

// LeftEvent is By leaving the game; its cards are played for it from then on.
type LeftEvent struct {
	By game.Seat `json:"by"`
}

// EventKind implements game.Event.
func (HandStartedEvent) EventKind() game.EventKind { return EventHandStarted }

// EventKind implements game.Event.
func (PassChosenEvent) EventKind() game.EventKind { return EventPassChosen }

// EventKind implements game.Event.
func (PassedEvent) EventKind() game.EventKind { return EventPassed }

// EventKind implements game.Event.
func (PlayedEvent) EventKind() game.EventKind { return EventPlayed }

// EventKind implements game.Event.
func (TrickWonEvent) EventKind() game.EventKind { return EventTrickWon }

// EventKind implements game.Event.
func (HandScoredEvent) EventKind() game.EventKind { return EventHandScored }

// EventKind implements game.Event.
func (LeftEvent) EventKind() game.EventKind { return EventLeft }
