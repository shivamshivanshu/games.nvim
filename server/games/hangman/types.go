package hangman

import "github.com/shivamshivanshu/games.nvim/server/game"

// TypeHangman is the game type on the wire.
const TypeHangman game.GameType = "hangman"

// SeatSolver is the only seat.
const SeatSolver game.Seat = "solver"

// MaxMisses is how many wrong guesses hang the solver.
const MaxMisses = 6

// The ways a game ends other than giving up.
const (
	ReasonSolved game.ResultReason = "solved"
	ReasonHanged game.ResultReason = "hanged"
)

// Opts is empty: Hangman has no options.
type Opts struct{}

// ActionGuess submits Text, one letter or the whole word, as the next guess.
const ActionGuess game.ActionKind = "guess"

// Action is typed input, sent raw and parsed by the server.
type Action struct {
	Kind game.ActionKind `json:"kind"`
	Text string          `json:"text,omitempty"`
}

// LetterGuess is a guessed letter and whether the word holds it.
type LetterGuess struct {
	Letter string `json:"letter"`
	Hit    bool   `json:"hit"`
}

// View shows the word as found so far. Pattern holds each letter of the word once revealed, ""
// while hidden; Guessed and MissedWords are oldest first; Misses counts both kinds of wrong guess.
// Word stays empty until the game finishes.
type View struct {
	Pattern     []string      `json:"pattern"`
	Guessed     []LetterGuess `json:"guessed"`
	MissedWords []string      `json:"missed_words"`
	Misses      int           `json:"misses"`
	Remaining   int           `json:"remaining"`
	Word        string        `json:"word,omitempty"`
}

// GuessedEvent reports a guess, a letter or a whole word, and whether it hit.
type GuessedEvent struct {
	By    game.Seat `json:"by"`
	Guess string    `json:"guess"`
	Hit   bool      `json:"hit"`
}

// EventGuessed is the kind of GuessedEvent.
const EventGuessed game.EventKind = "guessed"

// EventKind implements game.Event.
func (GuessedEvent) EventKind() game.EventKind { return EventGuessed }
