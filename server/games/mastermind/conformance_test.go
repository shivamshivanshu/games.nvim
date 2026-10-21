package mastermind

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

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

type persistedStep struct {
	seat   game.Seat
	action json.RawMessage
}

func TestRestoreReplaysPersistedStepsAndPlaysOnIdentically(t *testing.T) {
	const seed game.Seed = 5
	const restoreAt = 4
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
	secret := newState(t, seed).secret
	var steps []persistedStep
	for _, code := range append(wrongCodes(secret, MaxGuesses-1), string(secret)) {
		raw, err := json.Marshal(guess(code))
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
