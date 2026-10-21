package mastermind

import (
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

func guessSteps(codes ...string) []gametest.Step[Action] {
	steps := make([]gametest.Step[Action], len(codes))
	for i, code := range codes {
		steps[i] = gametest.Step[Action]{Seat: game.SeatSolo, Action: guess(code)}
	}
	return steps
}

func rejectedStep(code string) gametest.Step[Action] {
	return gametest.Step[Action]{Seat: game.SeatSolo, Action: guess(code), WantErr: true}
}

func TestConformance(t *testing.T) {
	solved := game.WinFor(ReasonSolved, game.SeatSolo)
	failed := game.LossBy(ReasonOutOfGuesses)
	script := func(name string, seed game.Seed, want *game.Result, steps func(secret Code) []gametest.Step[Action]) gametest.Script[Opts, Action] {
		secret := newState(t, seed).secret
		return gametest.Script[Opts, Action]{
			Name: name, Challenger: solver, Seed: seed,
			Steps: steps(secret), WantResult: want, Secrets: []string{string(secret)},
		}
	}
	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		script("solved first try", 1, &solved, func(secret Code) []gametest.Step[Action] {
			return guessSteps(string(secret))
		}),
		script("solved on the last guess", 2, &solved, func(secret Code) []gametest.Step[Action] {
			return guessSteps(append(wrongCodes(secret, MaxGuesses-1), string(secret))...)
		}),
		script("rejected guesses cost nothing, then out of guesses", 3, &failed, func(secret Code) []gametest.Step[Action] {
			misses := wrongCodes(secret, MaxGuesses)
			steps := append([]gametest.Step[Action]{rejectedStep("rgbw"), rejectedStep("rgb")}, guessSteps(misses[:5]...)...)
			steps = append(steps, rejectedStep("rgbyo"))
			steps = append(steps, guessSteps(misses[5:]...)...)
			return append(steps, gametest.Step[Action]{Seat: game.SeatSolo, Action: guess(string(secret)), WantErr: true})
		}),
		script("unfinished", 4, nil, func(secret Code) []gametest.Step[Action] {
			return guessSteps(wrongCodes(secret, 3)...)
		}),
	})
}
