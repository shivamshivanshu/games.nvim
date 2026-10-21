package g2048

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

// playedOut moves the first movable direction at every step from seed's start, as the core
// would number the steps, until the board is stuck or limit moves are made.
func playedOut(t *testing.T, seed game.Seed, limit int) ([]Direction, state) {
	t.Helper()
	s := newState(t, seed)
	var moves []Direction
	for !s.finished && len(moves) < limit {
		d := s.board.movable()[0]
		s, _ = apply(t, s, move(string(d)), seed, game.Seq(len(moves)+1))
		moves = append(moves, d)
	}
	return moves, s
}

func moveSteps(moves []Direction) []gametest.Step[Action] {
	steps := make([]gametest.Step[Action], len(moves))
	for i, d := range moves {
		steps[i] = gametest.Step[Action]{Seat: SeatSolver, Action: move(string(d))}
	}
	return steps
}

func rejected(action Action) gametest.Step[Action] {
	return gametest.Step[Action]{Seat: SeatSolver, Action: action, WantErr: true}
}

func TestConformance(t *testing.T) {
	lost := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{}, Reason: ReasonNoMoves}
	const unlimited = 10000
	stuckMoves, _ := playedOut(t, 1, unlimited)
	withRejections := func(seed game.Seed) []gametest.Step[Action] {
		moves, _ := playedOut(t, seed, unlimited)
		start := newState(t, seed).board
		steps := []gametest.Step[Action]{rejected(move("north")), rejected(Action{Kind: "slide", Direction: "up"})}
		for _, d := range directions() {
			if next, _ := start.slid(d); next == start {
				steps = append(steps, rejected(move(string(d))))
			}
		}
		return append(steps, moveSteps(moves)...)
	}
	someMoves, _ := playedOut(t, 3, 5)
	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		{Name: "played until no move is left", Challenger: solver, Seed: 1, Steps: moveSteps(stuckMoves), WantResult: &lost},
		{Name: "rejected moves cost nothing", Challenger: solver, Seed: 2, Steps: withRejections(2), WantResult: &lost},
		{Name: "unfinished", Challenger: solver, Seed: 3, Steps: moveSteps(someMoves)},
	})
}

type persistedStep struct {
	seat   game.Seat
	action json.RawMessage
}

func TestRestoreReplaysPersistedStepsAndPlaysOnIdentically(t *testing.T) {
	const seed game.Seed = 5
	const restoreAt = 20
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
	moves, final := playedOut(t, seed, 10000)
	if len(moves) <= restoreAt {
		t.Fatalf("seed %d ends after %d moves, before the restore at %d", seed, len(moves), restoreAt)
	}
	var steps []persistedStep
	for _, d := range moves {
		raw, err := json.Marshal(move(string(d)))
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, persistedStep{seat: SeatSolver, action: raw})
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
	want, _ := final.Outcome()
	if got, finished := restored.Outcome(); !finished || !cmp.Equal(got, want) {
		t.Errorf("restored outcome = %+v, %t; want %+v", got, finished, want)
	}
}

func viewJSON(t *testing.T, st game.ErasedState) string {
	t.Helper()
	raw, err := st.View(SeatSolver)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
