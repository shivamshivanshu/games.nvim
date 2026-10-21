package wordle

import "github.com/shivamshivanshu/games.nvim/server/game"

// TypeWordle is the game type on the wire.
const TypeWordle game.GameType = "wordle"

// SeatSolver is the only seat.
const SeatSolver game.Seat = "solver"

// WordLength is the number of letters in every word.
const WordLength = 5

// MaxGuesses is how many valid guesses a game allows.
const MaxGuesses = 6

// The ways a game ends other than giving up.
const (
	ReasonSolved       game.ResultReason = "solved"
	ReasonOutOfGuesses game.ResultReason = "out_of_guesses"
)

// Opts is empty: Wordle has no options.
type Opts struct{}

// ActionGuess submits Text as the next guess.
const ActionGuess game.ActionKind = "guess"

// Action is typed input, sent raw and parsed by the server.
type Action struct {
	Kind game.ActionKind `json:"kind"`
	Text string          `json:"text,omitempty"`
}

// Mark is the feedback for one letter of a guess.
type Mark string

// The marks, from best to worst.
const (
	MarkHit     Mark = "hit"
	MarkPresent Mark = "present"
	MarkMiss    Mark = "miss"
)

// Feedback holds one Mark per letter of a guess, in order.
type Feedback [WordLength]Mark

// Guess is a valid guess and its feedback.
type Guess struct {
	Word     string   `json:"word"`
	Feedback Feedback `json:"feedback"`
}

// Keyboard is the best Mark each guessed letter has earned so far; a letter not yet guessed is absent.
type Keyboard map[string]Mark

// View lists the guesses so far, oldest first. Answer stays empty until the game finishes.
type View struct {
	Guesses   []Guess  `json:"guesses"`
	Keyboard  Keyboard `json:"keyboard"`
	Remaining int      `json:"remaining"`
	Answer    string   `json:"answer,omitempty"`
}

// GuessedEvent reports a valid guess and its feedback.
type GuessedEvent struct {
	By       game.Seat `json:"by"`
	Word     string    `json:"word"`
	Feedback Feedback  `json:"feedback"`
}

// EventGuessed is the kind of GuessedEvent.
const EventGuessed game.EventKind = "guessed"

// EventKind implements game.Event.
func (GuessedEvent) EventKind() game.EventKind { return EventGuessed }
