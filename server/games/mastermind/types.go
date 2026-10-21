package mastermind

import "github.com/shivamshivanshu/games.nvim/server/game"

// TypeMastermind is the game type on the wire.
const TypeMastermind game.GameType = "mastermind"

// SeatSolver is the only seat.
const SeatSolver game.Seat = "solver"

// Palette is the colours a peg may take, one lowercase letter each: red, green, blue, yellow,
// orange and purple.
const Palette = "rgbyop"

// CodeLength is the number of pegs in the secret and in every guess.
const CodeLength = 4

// MaxGuesses is how many guesses a game allows.
const MaxGuesses = 10

// The ways a game ends other than giving up.
const (
	ReasonSolved       game.ResultReason = "solved"
	ReasonOutOfGuesses game.ResultReason = "out_of_guesses"
)

// Opts is empty: Mastermind has no options.
type Opts struct{}

// ActionGuess submits Code as the next guess.
const ActionGuess game.ActionKind = "guess"

// Action is typed input, sent raw and parsed by the server.
type Action struct {
	Kind game.ActionKind `json:"kind"`
	Code string          `json:"code,omitempty"`
}

// Code is CodeLength pegs, each a letter of Palette.
type Code string

// Guess is a guess and its feedback: Black pegs are the right colour in the right place, White
// pegs the right colour in the wrong place.
type Guess struct {
	Code  Code `json:"code"`
	Black int  `json:"black"`
	White int  `json:"white"`
}

// View lists the guesses so far, oldest first. Secret stays empty until the game finishes.
type View struct {
	Palette    []string `json:"palette"`
	CodeLength int      `json:"code_length"`
	Guesses    []Guess  `json:"guesses"`
	Remaining  int      `json:"remaining"`
	Secret     Code     `json:"secret,omitempty"`
}

// GuessedEvent reports a guess and its feedback.
type GuessedEvent struct {
	By    game.Seat `json:"by"`
	Code  Code      `json:"code"`
	Black int       `json:"black"`
	White int       `json:"white"`
}

// EventGuessed is the kind of GuessedEvent.
const EventGuessed game.EventKind = "guessed"

// EventKind implements game.Event.
func (GuessedEvent) EventKind() game.EventKind { return EventGuessed }
