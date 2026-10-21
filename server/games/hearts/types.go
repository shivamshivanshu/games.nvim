package hearts

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
)

// TypeHearts is the game type on the wire.
const TypeHearts game.GameType = "hearts"

const (
	players     = 4
	handSize    = 13
	passSize    = 3
	queenPoints = 13
	moonPoints  = 26
	// ScoreLimit ends the match at the end of the hand in which a seat reaches it.
	ScoreLimit = 100
)

// ReasonScoreLimit ends the match when a seat's score reaches ScoreLimit; the lowest score wins.
const ReasonScoreLimit game.ResultReason = "score_limit"

// Opts is empty: four players, play to 100, no options.
type Opts struct{}

// The action kinds.
const (
	ActionPass game.ActionKind = "pass"
	ActionPlay game.ActionKind = "play"
)

const (
	cardsField = "cards"
	cardField  = "card"
)

// Action is a move. Pass takes Cards, the three cards to pass, e.g.
// "2c Qs Ah"; play takes Card, the card to play, e.g. "Qs". Ranks are
// 2-9, T, J, Q, K, A and suits c, d, h, s, in any case.
type Action struct {
	Kind  game.ActionKind `json:"kind"`
	Cards string          `json:"cards,omitempty"`
	Card  string          `json:"card,omitempty"`
}

// Phase is what the hand waits on.
type Phase string

// The phases.
const (
	// PhasePass is every seat choosing three cards to pass.
	PhasePass Phase = "pass"
	// PhasePlay is the tricks.
	PhasePlay Phase = "play"
)

// PassDirection is where a hand's passes go: left, right, across, then
// held, in turn from the first hand.
type PassDirection string

// The pass directions; left is the next seat in play order.
const (
	PassLeft   PassDirection = "left"
	PassRight  PassDirection = "right"
	PassAcross PassDirection = "across"
	PassHold   PassDirection = "hold"
)

// Prompt is what the viewer is to do now.
type Prompt string

// The prompts.
const (
	// PromptNone is a viewer with nothing to do.
	PromptNone Prompt = ""
	// PromptPass is choosing the cards to pass.
	PromptPass Prompt = "pass"
	// PromptWait is waiting for the others to choose theirs.
	PromptWait Prompt = "wait"
	// PromptLead is leading the trick.
	PromptLead Prompt = "lead"
	// PromptPlay is playing to the trick.
	PromptPlay Prompt = "play"
)

// View is the table as one seat sees it: its own hand and passes, and every
// trick played.
//
// Legal is the viewer's own, the cards it may play now, empty unless it is
// to play. MustPass is the cards it must still choose to pass, 0 once
// chosen. Passed is what it passed, or chose to, this hand; Received is what
// ReceivedFrom passed to it. Prompt is the viewer's own.
type View struct {
	HandNo       int           `json:"hand_no"`
	Pass         PassDirection `json:"pass"`
	Phase        Phase         `json:"phase"`
	ScoreLimit   int           `json:"score_limit"`
	Seats        []SeatView    `json:"seats"`
	Trick        []Play        `json:"trick"`
	LastTrick    *TrickResult  `json:"last_trick" jsonschema:"nullable"`
	HeartsBroken bool          `json:"hearts_broken"`
	MyHand       []cards.Card  `json:"my_hand"`
	Legal        []cards.Card  `json:"legal"`
	MustPass     int           `json:"must_pass"`
	PassTo       *game.Seat    `json:"pass_to" jsonschema:"nullable"`
	ReceivedFrom *game.Seat    `json:"received_from" jsonschema:"nullable"`
	Passed       []cards.Card  `json:"passed"`
	Received     []cards.Card  `json:"received"`
	LastHand     *HandScore    `json:"last_hand" jsonschema:"nullable"`
	Prompt       Prompt        `json:"prompt"`
}

// SeatView is one seat at the table, in play order. Score is its total from
// the hands before this one, Taken the points it has taken this hand.
// PassChosen is whether it has chosen its pass. Out is a seat that left the
// game, whose cards are played for it.
type SeatView struct {
	Seat       game.Seat `json:"seat"`
	Score      int       `json:"score"`
	Taken      int       `json:"taken"`
	PassChosen bool      `json:"pass_chosen"`
	Out        bool      `json:"out"`
}

// Play is a card played to a trick.
type Play struct {
	Seat game.Seat  `json:"seat"`
	Card cards.Card `json:"card"`
}

// TrickResult is a finished trick and who took it, for Points.
type TrickResult struct {
	Plays  []Play    `json:"plays"`
	Winner game.Seat `json:"winner"`
	Points int       `json:"points"`
}

// HandScore is what each seat scored in a finished hand. Moon is the seat
// that took every point, which scores 0 while every other seat scores 26.
type HandScore struct {
	HandNo int          `json:"hand_no"`
	Points []SeatPoints `json:"points"`
	Moon   *game.Seat   `json:"moon" jsonschema:"nullable"`
}

// SeatPoints is a seat's points.
type SeatPoints struct {
	Seat   game.Seat `json:"seat"`
	Points int       `json:"points"`
}
