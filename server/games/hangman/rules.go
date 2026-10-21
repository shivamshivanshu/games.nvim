// Package hangman is solo Hangman: find a hidden word letter by letter before six misses.
package hangman

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/words"
)

const guessField = "text"

// Rules implements game.Rules for Hangman.
type Rules struct{}

// Info describes Hangman: solo, no options.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeHangman,
		Name:         "Hangman",
		Players:      game.PlayerRange{Min: 1, Max: 1},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Actions: []game.ActionSpec{{
			Kind:        ActionGuess,
			Description: "Guess a letter, or the whole word.",
			Input: &game.ActionInput{
				Field:       guessField,
				Description: fmt.Sprintf("One letter, or a %d-letter word from the word list, in any case.", words.Length),
				Pattern:     guessPattern(),
			},
		}},
		Events:  []game.Event{GuessedEvent{}},
		Reasons: []game.ResultReason{ReasonSolved, ReasonHanged},
	}
}

// New seats the challenger alone and draws the word from the answer list with rng.
func (Rules) New(_ Opts, challenger game.UserID, _ []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	answers := words.Answers()
	return state{word: answers.At(rng.IntN(answers.Len()))}, game.SoloSeat(challenger), nil
}
