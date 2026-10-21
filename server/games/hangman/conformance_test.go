package hangman

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
	hanged := game.LossBy(ReasonHanged)
	script := func(name string, seed game.Seed, want *game.Result, steps func(word string) []gametest.Step[Action]) gametest.Script[Opts, Action] {
		word := newState(t, seed).word
		return gametest.Script[Opts, Action]{
			Name: name, Challenger: solver, Seed: seed,
			Steps: steps(word), WantResult: want, Secrets: []string{word},
		}
	}
	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		script("solved letter by letter", 1, &solved, func(word string) []gametest.Step[Action] {
			return guessSteps(append(lettersNotIn(word, MaxMisses-1), lettersOf(word)...)...)
		}),
		script("solved by the whole word", 2, &solved, func(word string) []gametest.Step[Action] {
			return guessSteps(append(wordsNot(word, 2), word[:1], word)...)
		}),
		script("rejected guesses cost nothing, then hanged", 3, &hanged, func(word string) []gametest.Step[Action] {
			misses := append(wordsNot(word, 2), lettersNotIn(word, MaxMisses-2)...)
			steps := append([]gametest.Step[Action]{rejectedStep("zzzzz"), rejectedStep("ab")}, guessSteps(word[:1])...)
			steps = append(steps, rejectedStep(word[:1]))
			steps = append(steps, guessSteps(misses[:3]...)...)
			steps = append(steps, rejectedStep(misses[0]), rejectedStep(misses[2]))
			steps = append(steps, guessSteps(misses[3:]...)...)
			return append(steps, gametest.Step[Action]{Seat: game.SeatSolo, Action: guess(word), WantErr: true})
		}),
		script("unfinished", 4, nil, func(word string) []gametest.Step[Action] {
			return guessSteps(word[:1], lettersNotIn(word, 1)[0])
		}),
	})
}
