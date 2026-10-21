// Package wordle is solo Wordle: find a hidden five-letter word in six guesses.
package wordle

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/words"
)

const guessField = "text"

// guessPattern is WordLength ASCII letters, with surrounding whitespace allowed.
func guessPattern() string { return fmt.Sprintf(`^\s*[A-Za-z]{%d}\s*$`, WordLength) }

// Rules implements game.Rules for Wordle.
type Rules struct{}

// Info describes Wordle: solo, no options.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeWordle,
		Name:         "Wordle",
		Players:      game.PlayerRange{Min: 1, Max: 1},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Actions: []game.ActionSpec{{
			Kind:        ActionGuess,
			Description: "Guess the hidden word.",
			Input: &game.ActionInput{
				Field:       guessField,
				Description: fmt.Sprintf("A %d-letter word from the word list, in any case.", WordLength),
				Pattern:     guessPattern(),
			},
		}},
		Events:  []game.Event{GuessedEvent{}},
		Reasons: []game.ResultReason{ReasonSolved, ReasonOutOfGuesses},
	}
}

// New seats the challenger alone and draws the answer from rng.
func (Rules) New(_ Opts, challenger game.UserID, _ []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	answers := words.Answers()
	answer := answers.At(rng.IntN(answers.Len()))
	return state{answer: answer}, game.SoloSeat(challenger), nil
}
