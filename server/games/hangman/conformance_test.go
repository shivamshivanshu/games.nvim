package hangman

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

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

type persistedStep struct {
	seat   game.Seat
	action json.RawMessage
}

func TestRestoreReplaysPersistedStepsAndPlaysOnIdentically(t *testing.T) {
	const seed game.Seed = 5
	const restoreAt = 3
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	start := func() game.ErasedState {
		st, _, err := g.New(opts, solver, nil, game.StepRNG(seed, 0))
		if err != nil {
			t.Fatal(err)
		}
		return st
	}
	apply := func(st game.ErasedState, step persistedStep, seq game.Seq) game.ErasedState {
		next, _, err := st.Apply(step.seat, step.action, game.StepRNG(seed, seq))
		if err != nil {
			t.Fatalf("seq %d: %v", seq, err)
		}
		return next
	}
	word := newState(t, seed).word
	var steps []persistedStep
	for _, text := range append(lettersNotIn(word, 2), lettersOf(word)...) {
		raw, err := json.Marshal(guess(text))
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, persistedStep{seat: game.SeatSolo, action: raw})
	}

	original := start()
	for i, step := range steps[:restoreAt] {
		original = apply(original, step, game.Seq(i+1))
	}
	restored := start()
	for i, step := range steps[:restoreAt] {
		restored = apply(restored, step, game.Seq(i+1))
	}
	if diff := cmp.Diff(viewJSON(t, original), viewJSON(t, restored)); diff != "" {
		t.Fatalf("restored view differs (-original +restored):\n%s", diff)
	}
	for i, step := range steps[restoreAt:] {
		seq := game.Seq(restoreAt + i + 1)
		original, restored = apply(original, step, seq), apply(restored, step, seq)
		if diff := cmp.Diff(viewJSON(t, original), viewJSON(t, restored)); diff != "" {
			t.Fatalf("seq %d: restored view differs (-original +restored):\n%s", seq, diff)
		}
	}
	want := game.WinFor(ReasonSolved, game.SeatSolo)
	if got, finished := restored.Outcome(); !finished || !cmp.Equal(got, want) {
		t.Errorf("restored outcome = %+v, %t; want %+v", got, finished, want)
	}
}

func viewJSON(t *testing.T, st game.ErasedState) string {
	t.Helper()
	raw, err := st.View(game.SeatSolo)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
