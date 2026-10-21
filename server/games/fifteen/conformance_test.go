package fifteen

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

func newGame(t *testing.T, size Size, seed game.Seed) state {
	t.Helper()
	st, _, err := Rules{}.New(Opts{Size: size}, solo, nil, game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	return st.(state)
}

// solvingActions solves the seed's shuffle, each slide sent as a move by direction when
// the tile's index in the solution is even, and by number when odd.
func solvingActions(t *testing.T, size Size, seed game.Seed) []Action {
	t.Helper()
	s := newGame(t, size, seed)
	var actions []Action
	for i, tile := range solve(t, s.board) {
		action := slide(tile)
		if i%2 == 0 {
			for _, m := range s.board.movable() {
				if m.Tile == tile {
					action = move(m.Direction)
				}
			}
		}
		actions = append(actions, action)
		s.board = s.board.slid(tile)
	}
	return actions
}

func steps(actions []Action) []gametest.Step[Action] {
	out := make([]gametest.Step[Action], len(actions))
	for i, action := range actions {
		out[i] = gametest.Step[Action]{Seat: SeatSolver, Action: action}
	}
	return out
}

func refused(action Action) gametest.Step[Action] {
	return gametest.Step[Action]{Seat: SeatSolver, Action: action, WantErr: true}
}

const conformanceSeed game.Seed = 4

func TestConformance(t *testing.T) {
	solved := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonSolved}
	var scripts []gametest.Script[Opts, Action]
	for _, size := range sizes() {
		s := newGame(t, size, conformanceSeed)
		stuck := Action{Kind: ActionSlide, Tile: "99"}
		play := append([]gametest.Step[Action]{refused(stuck), refused(Action{Kind: ActionMove, Direction: "north"})},
			steps(solvingActions(t, size, conformanceSeed))...)
		play = append(play, refused(slide(s.board.movable()[0].Tile)))
		scripts = append(scripts, gametest.Script[Opts, Action]{
			Name: "solves " + string(size), Opts: Opts{Size: size}, Challenger: solo, Seed: conformanceSeed,
			Steps: play, WantResult: &solved,
		})
	}
	scripts = append(scripts, gametest.Script[Opts, Action]{
		Name: "unfinished", Opts: Opts{Size: Size4}, Challenger: solo, Seed: 5,
		Steps: steps(solvingActions(t, Size4, 5)[:3]),
	})
	gametest.Run(t, Rules{}, scripts)
}

type persistedStep struct {
	seat   game.Seat
	action json.RawMessage
}

func TestRestoreReplaysPersistedStepsAndPlaysOnIdentically(t *testing.T) {
	const seed game.Seed = 6
	const restoreAt = 5
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
	for _, action := range solvingActions(t, Size4, seed) {
		raw, err := json.Marshal(action)
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
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonSolved}
	if got, finished := restored.Outcome(); !finished || !cmp.Equal(got, want) {
		t.Errorf("restored outcome = %+v, %t; want %+v", got, finished, want)
	}
}
