package wordle

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
	answer := newState(t, seed).answer
	var steps []persistedStep
	for _, text := range append(wrongGuesses(answer, MaxGuesses-1), answer) {
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
