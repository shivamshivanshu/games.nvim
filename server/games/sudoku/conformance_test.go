package sudoku

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

func newState(t *testing.T, opts Opts, seed game.Seed) state {
	t.Helper()
	st, _, err := Rules{}.New(opts, solo, nil, game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	return st.(state)
}

func setText(cell int, d Digit) string {
	return fmt.Sprintf("%d%d %d", cell/Size+1, cell%Size+1, d)
}

// solvingSets fills every open cell of s with its solution digit, in row-major order.
func solvingSets(s state) []string {
	var texts []string
	for cell, d := range s.puzzle {
		if d == Empty {
			texts = append(texts, setText(cell, s.solution[cell]))
		}
	}
	return texts
}

func digitsOf(g Grid) string {
	var b strings.Builder
	for _, d := range g {
		b.WriteByte(byte('0' + d))
	}
	return b.String()
}

// solvingScript makes a wrong set, clears it, solves, then tries a set after the finish,
// around two illegal sets at the start.
func solvingScript(t *testing.T, d Difficulty, seed game.Seed) gametest.Script[Opts, Action] {
	t.Helper()
	opts := Opts{Difficulty: d}
	s := newState(t, opts, seed)
	given := slices.IndexFunc(s.puzzle[:], func(d Digit) bool { return d != Empty })
	open := slices.Index(s.puzzle[:], Empty)
	steps := []gametest.Step[Action]{
		{Seat: SeatSolver, Action: set(setText(given, Empty)), WantErr: true},
		{Seat: SeatSolver, Action: set("00 1"), WantErr: true},
		{Seat: SeatSolver, Action: set(setText(open, s.solution[open]%Size+1))},
		{Seat: SeatSolver, Action: set(setText(open, Empty))},
	}
	for _, text := range solvingSets(s) {
		steps = append(steps, gametest.Step[Action]{Seat: SeatSolver, Action: set(text)})
	}
	steps = append(steps, gametest.Step[Action]{Seat: SeatSolver, Action: set(setText(open, Empty)), WantErr: true})
	return gametest.Script[Opts, Action]{
		Name:       string(d),
		Opts:       opts,
		Challenger: solo,
		Seed:       seed,
		Steps:      steps,
		WantResult: &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonSolved},
		Secrets:    []string{digitsOf(s.solution)},
	}
}

func difficulties() []Difficulty {
	return []Difficulty{DifficultyEasy, DifficultyMedium, DifficultyHard}
}

const conformanceSeed game.Seed = 3

func TestConformance(t *testing.T) {
	var scripts []gametest.Script[Opts, Action]
	for _, d := range difficulties() {
		scripts = append(scripts, solvingScript(t, d, conformanceSeed))
	}
	gametest.Run(t, Rules{}, scripts)
}

func TestSolutionStaysHiddenAtEveryUnfinishedStep(t *testing.T) {
	for _, d := range difficulties() {
		script := solvingScript(t, d, conformanceSeed)
		var st game.State[Action, View] = newState(t, script.Opts, script.Seed)
		for i, step := range script.Steps {
			if len(st.ToAct()) == 0 {
				break
			}
			if got := st.View(SeatSolver).Solution; len(got) != 0 {
				t.Fatalf("%s before step %d: unfinished view shows solution %v", d, i, got)
			}
			next, _, err := st.Apply(step.Seat, step.Action, nil)
			if (err != nil) != step.WantErr {
				t.Fatalf("%s step %d: err = %v, want error %t", d, i, err, step.WantErr)
			}
			if err == nil {
				st = next
			}
		}
		if _, finished := st.Outcome(); !finished {
			t.Errorf("%s: the script did not finish the game", d)
		}
	}
}

type persistedStep struct {
	seat   game.Seat
	action json.RawMessage
}

func TestReplayFromSeedAndPersistedStepsPlaysOnIdentically(t *testing.T) {
	const seed game.Seed = 11
	const restoreAt = 20
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	start := func() game.ErasedState {
		st, _, err := g.New(opts, solo, nil, game.StepRNG(seed, 0))
		if err != nil {
			t.Fatal(err)
		}
		return st
	}
	apply := func(st game.ErasedState, step persistedStep, seq game.Seq) (game.ErasedState, []json.RawMessage) {
		next, events, err := st.Apply(step.seat, step.action, game.StepRNG(seed, seq))
		if err != nil {
			t.Fatalf("seq %d: %v", seq, err)
		}
		return next, events
	}
	var plan []persistedStep
	for _, text := range solvingSets(newState(t, Opts{Difficulty: DifficultyMedium}, seed)) {
		raw, err := json.Marshal(set(text))
		if err != nil {
			t.Fatal(err)
		}
		plan = append(plan, persistedStep{seat: SeatSolver, action: raw})
	}

	live := start()
	for i, step := range plan[:restoreAt] {
		live, _ = apply(live, step, game.Seq(i)+1)
	}
	restored := start()
	for i, step := range plan[:restoreAt] {
		restored, _ = apply(restored, step, game.Seq(i)+1)
	}

	for i, step := range plan[restoreAt:] {
		seq := game.Seq(restoreAt+i) + 1
		liveView, _ := live.View(SeatSolver)
		restoredView, _ := restored.View(SeatSolver)
		if diff := cmp.Diff(string(liveView), string(restoredView)); diff != "" {
			t.Fatalf("seq %d: restored view differs (-live +restored):\n%s", seq, diff)
		}
		var liveEvents, restoredEvents []json.RawMessage
		live, liveEvents = apply(live, step, seq)
		restored, restoredEvents = apply(restored, step, seq)
		if diff := cmp.Diff(liveEvents, restoredEvents); diff != "" {
			t.Fatalf("seq %d: restored events differ (-live +restored):\n%s", seq, diff)
		}
	}
	liveResult, _ := live.Outcome()
	restoredResult, finished := restored.Outcome()
	if diff := cmp.Diff(liveResult, restoredResult); !finished || diff != "" {
		t.Errorf("restored finished=%t, result (-live +restored):\n%s", finished, diff)
	}
}
