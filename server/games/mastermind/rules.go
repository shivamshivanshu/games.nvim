// Package mastermind is solo Mastermind: find a hidden code of coloured pegs from black and
// white feedback.
package mastermind

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const codeField = "code"

// Rules implements game.Rules for Mastermind.
type Rules struct{}

// Info describes Mastermind: solo, no options.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeMastermind,
		Name:         "Mastermind",
		Players:      game.PlayerRange{Min: 1, Max: 1},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Actions: []game.ActionSpec{{
			Kind:        ActionGuess,
			Description: "Guess the hidden code.",
			Input: &game.ActionInput{
				Field:       codeField,
				Description: fmt.Sprintf("%d peg colours from %s (red, green, blue, yellow, orange, purple), in any case.", CodeLength, Palette),
				Pattern:     codePattern(),
			},
		}},
		Events:  []game.Event{GuessedEvent{}},
		Reasons: []game.ResultReason{ReasonSolved, ReasonOutOfGuesses},
	}
}

// New seats the challenger alone and draws the secret from rng; colours may repeat.
func (Rules) New(_ Opts, challenger game.UserID, _ []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	secret := make([]byte, CodeLength)
	for i := range secret {
		secret[i] = Palette[rng.IntN(len(Palette))]
	}
	return state{secret: Code(secret)}, game.SoloSeat(challenger), nil
}
