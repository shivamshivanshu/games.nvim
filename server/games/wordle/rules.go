// Package wordle is solo Wordle: find a hidden five-letter word in six guesses.
package wordle

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const (
	guessField   = "text"
	guessPattern = `^\s*[A-Za-z]{5}\s*$`
)

// Rules implements game.Rules for Wordle.
type Rules struct{}

// EventTypes implements game.EventCatalog.
func (Rules) EventTypes() []game.Event { return []game.Event{GuessedEvent{}} }

// Reasons implements game.ReasonCatalog.
func (Rules) Reasons() []game.ResultReason {
	return []game.ResultReason{ReasonSolved, ReasonOutOfGuesses}
}

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
				Description: "A five-letter word from the word list, in any case.",
				Pattern:     guessPattern,
			},
		}},
	}
}

// New seats the challenger alone and draws the answer from rng.
func (Rules) New(_ Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	if len(invitees) != 0 {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("wordle is solo, got %d opponents", len(invitees))}
	}
	answers := answerWords()
	answer := answers.at(rng.IntN(answers.len()))
	return state{answer: answer}, map[game.UserID]game.Seat{challenger: SeatSolver}, nil
}
