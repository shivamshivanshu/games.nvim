package wordle

import (
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

func guessSteps(texts ...string) []gametest.Step[Action] {
	steps := make([]gametest.Step[Action], len(texts))
	for i, text := range texts {
		steps[i] = gametest.Step[Action]{Seat: game.SeatSolo, Action: guess(text)}
	}
	return steps
}

func rejectedStep(text string) gametest.Step[Action] {
	return gametest.Step[Action]{Seat: game.SeatSolo, Action: guess(text), WantErr: true}
}

func TestConformance(t *testing.T) {
	solved := game.WinFor(ReasonSolved, game.SeatSolo)
	failed := game.LossBy(ReasonOutOfGuesses)
	script := func(name string, seed game.Seed, want *game.Result, steps func(answer string) []gametest.Step[Action]) gametest.Script[Opts, Action] {
		answer := newState(t, seed).answer
		return gametest.Script[Opts, Action]{
			Name: name, Challenger: solver, Seed: seed,
			Steps: steps(answer), WantResult: want, Secrets: []string{answer},
		}
	}
	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		script("solved first try", 1, &solved, func(answer string) []gametest.Step[Action] {
			return guessSteps(answer)
		}),
		script("solved on the last guess", 2, &solved, func(answer string) []gametest.Step[Action] {
			return guessSteps(append(wrongGuesses(answer, MaxGuesses-1), answer)...)
		}),
		script("rejected guesses cost nothing, then out of guesses", 3, &failed, func(answer string) []gametest.Step[Action] {
			misses := wrongGuesses(answer, MaxGuesses)
			steps := append([]gametest.Step[Action]{rejectedStep("zzzzz"), rejectedStep("abc")}, guessSteps(misses[:3]...)...)
			steps = append(steps, rejectedStep("qqqqq"))
			steps = append(steps, guessSteps(misses[3:]...)...)
			return append(steps, gametest.Step[Action]{Seat: game.SeatSolo, Action: guess(answer), WantErr: true})
		}),
		script("unfinished", 4, nil, func(answer string) []gametest.Step[Action] {
			return guessSteps(wrongGuesses(answer, 2)...)
		}),
	})
}
