package ludo

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const maxGameSteps = 5000

func players() []game.UserID { return []game.UserID{"alice", "bob", "carol", "dave"} }

// firstChoice is a deterministic player: roll when due, else move the lowest movable token.
func firstChoice(v View) Action {
	if v.Phase == PhaseRoll {
		return Action{Kind: ActionRoll}
	}
	return Action{Kind: ActionMove, Token: v.Movable[0]}
}

// playToFinish plays firstChoice for every seat with the rng each step would get in the server.
func playToFinish(t *testing.T, users []game.UserID, seed game.Seed) []gametest.Step[Action] {
	t.Helper()
	st, _, err := Rules{}.New(Opts{}, users[0], users[1:], game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	var steps []gametest.Step[Action]
	for seq := game.Seq(1); len(st.ToAct()) > 0; seq++ {
		if seq > maxGameSteps {
			t.Fatalf("%d players: no finish within %d steps", len(users), maxGameSteps)
		}
		seat := st.ToAct()[0]
		action := firstChoice(st.View(seat))
		if st, _, err = st.Apply(seat, action, game.StepRNG(seed, seq)); err != nil {
			t.Fatal(err)
		}
		steps = append(steps, gametest.Step[Action]{Seat: seat, Action: action})
	}
	return steps
}

func notFirst(first game.Seat) game.Seat {
	if first == SeatBlue {
		return SeatYellow
	}
	return SeatBlue
}

func TestConformance(t *testing.T) {
	var scripts []gametest.Script[Opts, Action]
	for n := minPlayers; n <= maxPlayers; n++ {
		const seed game.Seed = 7
		users := players()[:n]
		steps := playToFinish(t, users, seed)
		winner := steps[len(steps)-1].Seat
		outOfTurn := gametest.Step[Action]{Seat: notFirst(steps[0].Seat), Action: Action{Kind: ActionRoll}, WantErr: true}
		scripts = append(scripts, gametest.Script[Opts, Action]{
			Name:       strconv.Itoa(n) + " players",
			Challenger: users[0],
			Invitees:   users[1:],
			Seed:       seed,
			Steps:      append([]gametest.Step[Action]{outOfTurn}, steps...),
			WantResult: &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{winner}, Reason: ReasonAllHome},
		})
	}
	gametest.Run(t, Rules{}, scripts)
}

type persistedStep struct {
	seat   game.Seat
	action json.RawMessage
}

func TestRestoreReplaysPersistedStepsAndPlaysOnIdentically(t *testing.T) {
	const (
		seed      game.Seed = 11
		restoreAt game.Seq  = 40
	)
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	start := func() game.ErasedState {
		st, _, err := g.New(opts, players()[0], players()[1:3], game.StepRNG(seed, 0))
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
	choose := func(st game.ErasedState) persistedStep {
		seat := st.ToAct()[0]
		raw, err := st.View(seat)
		if err != nil {
			t.Fatal(err)
		}
		var view View
		if err := json.Unmarshal(raw, &view); err != nil {
			t.Fatal(err)
		}
		action, err := json.Marshal(firstChoice(view))
		if err != nil {
			t.Fatal(err)
		}
		return persistedStep{seat: seat, action: action}
	}

	live := start()
	var persisted []persistedStep
	for seq := game.Seq(1); seq <= restoreAt; seq++ {
		step := choose(live)
		live, _ = apply(live, step, seq)
		persisted = append(persisted, step)
	}
	if _, finished := live.Outcome(); finished {
		t.Fatalf("game finished before the restore point %d", restoreAt)
	}
	restored := start()
	for i, step := range persisted {
		restored, _ = apply(restored, step, game.Seq(i)+1)
	}

	for seq := restoreAt + 1; seq <= maxGameSteps; seq++ {
		liveView, _ := live.View(SeatBlue)
		restoredView, _ := restored.View(SeatBlue)
		if diff := cmp.Diff(string(liveView), string(restoredView)); diff != "" {
			t.Fatalf("seq %d: restored view differs (-live +restored):\n%s", seq, diff)
		}
		if len(live.ToAct()) == 0 {
			break
		}
		step := choose(live)
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
