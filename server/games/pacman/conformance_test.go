package pacman

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

const unlimited = 100_000

func TestConformance(t *testing.T) {
	lost := game.LossBy(ReasonGameOver)
	won := game.WinFor(ReasonCleared, SeatPlayer)
	lostSteps, lostEnd := autoplay(t, 0, unlimited)
	wonSteps, wonEnd := autoplay(t, 3, unlimited)
	lostResult, _ := lostEnd.Outcome()
	wonResult, _ := wonEnd.Outcome()
	if lostResult.Reason != ReasonGameOver || wonResult.Reason != ReasonCleared {
		t.Fatalf("seed 0 ends %+v and seed 3 %+v; want a loss and a win", lostResult, wonResult)
	}
	tick := gametest.Step[Action]{Kind: gametest.StepTick}
	rejected := func(a Action) gametest.Step[Action] {
		return gametest.Step[Action]{Seat: SeatPlayer, Action: a, WantErr: true}
	}
	withPauses := []gametest.Step[Action]{
		rejected(Action{Kind: ActionPause}),
		rejected(Action{Kind: ActionTurn, Direction: "north"}),
		rejected(Action{Kind: "jump"}),
		{Seat: SeatPlayer, Action: turn(grid.Left)},
		tick,
		{Seat: SeatPlayer, Action: Action{Kind: ActionPause}},
		{Seat: SeatPlayer, Action: turn(grid.Right)},
		tick,
		{Kind: gametest.StepPause},
		rejected(Action{Kind: ActionPause}),
		{Seat: SeatPlayer, Action: turn(grid.Up)},
		tick, tick, tick,
	}
	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		{Name: "caught on every life", Challenger: player, Seed: 0, Steps: lostSteps, WantResult: &lost},
		{Name: "every level cleared", Challenger: player, Seed: 3, Steps: wonSteps, WantResult: &won},
		{Name: "pauses and rejected actions, unfinished", Challenger: player, Seed: 5, Steps: withPauses},
	})
}

type persistedStep struct {
	kind   gametest.StepKind
	action json.RawMessage
}

func TestRestoreReplaysTicksAndTurnsIdentically(t *testing.T) {
	const seed game.Seed = 3
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	start := func() game.ErasedState {
		st, _, err := g.New(opts, player, nil, game.StepRNG(seed, 0))
		if err != nil {
			t.Fatal(err)
		}
		return st
	}
	apply := func(st game.ErasedState, step persistedStep, seq game.Seq) game.ErasedState {
		var next game.ErasedState
		var err error
		if step.kind == gametest.StepTick {
			next, _, err = st.Tick(game.StepRNG(seed, seq))
		} else {
			next, _, err = st.Apply(SeatPlayer, step.action, game.StepRNG(seed, seq))
		}
		if err != nil {
			t.Fatalf("seq %d: %v", seq, err)
		}
		return next
	}
	played, final := autoplay(t, seed, unlimited)
	var steps []persistedStep
	for _, st := range played {
		raw, err := json.Marshal(st.Action)
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, persistedStep{kind: st.Kind, action: raw})
	}
	restoreAt := len(steps) / 2

	original := start()
	for i, step := range steps[:restoreAt] {
		original = apply(original, step, game.Seq(i+1))
	}
	restored := start()
	for i, step := range steps[:restoreAt] {
		restored = apply(restored, step, game.Seq(i+1))
	}
	frightened := 0
	for i, step := range steps[restoreAt:] {
		seq := game.Seq(restoreAt + i + 1)
		original, restored = apply(original, step, seq), apply(restored, step, seq)
		if diff := cmp.Diff(viewJSON(t, original), viewJSON(t, restored)); diff != "" {
			t.Fatalf("seq %d: restored view differs (-original +restored):\n%s", seq, diff)
		}
		if strings.Contains(viewJSON(t, restored), `"mode":"frightened"`) {
			frightened++
		}
	}
	if frightened == 0 {
		t.Error("no ghost was frightened after the restore, so the replay drew nothing from the rng")
	}
	want, _ := final.Outcome()
	if got, finished := restored.Outcome(); !finished || !cmp.Equal(got, want) {
		t.Errorf("restored outcome = %+v, %t; want %+v", got, finished, want)
	}
}

func viewJSON(t *testing.T, st game.ErasedState) string {
	t.Helper()
	raw, err := st.View(SeatPlayer)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
