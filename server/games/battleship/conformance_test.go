package battleship

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

// planner plays a game step by step with the core's step rngs, so a script
// knows where random fill put a fleet before it fires at it.
type planner struct {
	t     *testing.T
	seed  game.Seed
	state game.State[Action, View]
	steps []gametest.Step[Action]
}

func newPlanner(t *testing.T, first game.FirstMover, seed game.Seed) *planner {
	t.Helper()
	st, _, err := Rules{}.New(Opts{First: first}, alice, []game.UserID{bob}, game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	return &planner{t: t, seed: seed, state: st}
}

func (p *planner) applied() game.Seq {
	n := game.Seq(0)
	for _, step := range p.steps {
		if !step.WantErr {
			n++
		}
	}
	return n
}

func (p *planner) play(seat game.Seat, actions ...Action) {
	p.t.Helper()
	for _, action := range actions {
		next, _, err := p.state.Apply(seat, action, game.StepRNG(p.seed, p.applied()+1))
		if err != nil {
			p.t.Fatalf("plan %s %+v: %v", seat, action, err)
		}
		p.state = next
		p.steps = append(p.steps, gametest.Step[Action]{Seat: seat, Action: action})
	}
}

func (p *planner) refuse(seat game.Seat, action Action) {
	p.steps = append(p.steps, gametest.Step[Action]{Seat: seat, Action: action, WantErr: true})
}

// sink has the seat to fire shoot at every ship cell of its opponent's, while
// the opponent answers each shot but the last at the next cell of its own fire list.
func (p *planner) sink(shooter game.Seat, answers []string) {
	p.t.Helper()
	opponent := p.state.(state).opponent(shooter)
	targets := shipNames(p.state, opponent)
	for i, cell := range targets {
		p.play(shooter, fire(cell))
		if i < len(targets)-1 {
			p.play(opponent, fire(answers[i]))
		}
	}
}

func (p *planner) script(name string, first game.FirstMover, want *game.Result) gametest.Script[Opts, Action] {
	return gametest.Script[Opts, Action]{
		Name: name, Opts: Opts{First: first}, Challenger: alice, Invitees: []game.UserID{bob},
		Seed: p.seed, Steps: p.steps, WantResult: want,
	}
}

// bottomRows are cells of rows 9 and 10, which neither fixed fleet's p1 shots need.
func bottomRows() []string {
	var cells []string
	for _, row := range []string{"10", "9"} {
		for col := 'a'; col <= 'j'; col++ {
			cells = append(cells, string(col)+row)
		}
	}
	return cells
}

// fullGame has p2 place by hand after refused tries and p1 fill at random,
// then p1 sinks p2's fleet.
func fullGame(t *testing.T, first game.FirstMover, seed game.Seed) *planner {
	p := newPlanner(t, first, seed)
	p.refuse(p2, fire("a1"))
	p.play(p2, down("a1"))
	p.refuse(p2, down("a1"))
	p.refuse(p2, across("h5"))
	p.play(p2, columnsFleet()[1:]...)
	p.refuse(p2, random())
	p.play(p1, random())
	p.refuse(p2, fire("a1"))
	p.sink(p1, bottomRows())
	return p
}

func TestConformance(t *testing.T) {
	p1Wins := &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{p1}, Reason: ReasonFleetSunk}
	p2Wins := &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{p2}, Reason: ReasonFleetSunk}

	bothRandom := newPlanner(t, game.FirstMoverOpponent, 3)
	bothRandom.play(p1, random())
	bothRandom.play(p2, random())
	bothRandom.play(p1, fire("j10"))
	bothRandom.sink(p2, []string{"a1", "b1", "c1", "d1", "e1", "f1", "g1", "h1", "i1", "j1", "a2", "b2", "c2", "d2", "e2", "f2"})

	setup := newPlanner(t, game.FirstMoverRandom, 5)
	setup.play(p1, across("c3"))
	setup.play(p2, random())

	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		fullGame(t, game.FirstMoverMe, 1).script("p1 sinks a fleet placed by hand", game.FirstMoverMe, p1Wins),
		bothRandom.script("both fill at random, p2 sinks p1", game.FirstMoverOpponent, p2Wins),
		setup.script("still placing", game.FirstMoverRandom, nil),
	})
}

// TestTheOpponentsUnhitShipsNeverReachTheOtherSeat plays one game twice with
// p1's submarine and destroyer in different places that no shot reaches, and
// requires every view of p2's and every event to be the same in both.
func TestTheOpponentsUnhitShipsNeverReachTheOtherSeat(t *testing.T) {
	fleets := [][]Action{
		rowsFleet(),
		{across("a1"), across("a3"), across("a5"), down("j1"), across("f9")},
	}
	shots := []move{
		{p1, fire("a1")}, {p2, fire("a1")}, {p1, fire("c3")}, {p2, fire("b1")},
		{p1, fire("j10")}, {p2, fire("c1")}, {p1, fire("b4")}, {p2, fire("d1")},
		{p1, fire("b5")}, {p2, fire("e1")}, {p1, fire("h8")}, {p2, fire("b3")},
		{p1, fire("a2")}, {p2, fire("e7")},
	}
	type seen struct {
		Views  []View
		Events [][]json.RawMessage
	}
	play := func(fleet []Action) seen {
		g := game.Adapt(Rules{})
		opts, err := g.ResolveOptions(json.RawMessage(`{"first":"me"}`))
		if err != nil {
			t.Fatal(err)
		}
		st, _, err := g.New(opts, alice, []game.UserID{bob}, game.StepRNG(1, 0))
		if err != nil {
			t.Fatal(err)
		}
		ms := append(append(moves(p2, columnsFleet()[:2]...), moves(p1, fleet...)...), moves(p2, columnsFleet()[2:]...)...)
		ms = append(ms, shots...)
		var out seen
		for i, m := range ms {
			raw, err := json.Marshal(m.action)
			if err != nil {
				t.Fatal(err)
			}
			next, events, err := st.Apply(m.seat, raw, game.StepRNG(1, game.Seq(i+1)))
			if err != nil {
				t.Fatalf("move %d %+v: %v", i, m, err)
			}
			st = next
			rawView, err := st.View(p2)
			if err != nil {
				t.Fatal(err)
			}
			var view View
			if err := json.Unmarshal(rawView, &view); err != nil {
				t.Fatal(err)
			}
			out.Views = append(out.Views, view)
			out.Events = append(out.Events, events)
		}
		if _, finished := st.Outcome(); finished {
			t.Fatal("the game finished, which reveals the fleets")
		}
		return out
	}
	first, second := play(fleets[0]), play(fleets[1])
	if diff := cmp.Diff(first, second); diff != "" {
		t.Errorf("p2 can tell where p1's unhit ships lie (-first +second):\n%s", diff)
	}
	if last := first.Views[len(first.Views)-1]; last.Target[0][0] != TileSunk || last.Target[2][1] != TileHit || last.Target[6][4] != TileMiss {
		t.Fatal("the shots did not play as planned")
	}
}

type persistedStep struct {
	seat   game.Seat
	action json.RawMessage
}

func TestRestoreReplaysRandomFleetsAndPlaysOnIdentically(t *testing.T) {
	const seed game.Seed = 11
	const restoreAt = 3
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(json.RawMessage(`{"first":"random"}`))
	if err != nil {
		t.Fatal(err)
	}
	start := func() game.ErasedState {
		st, _, err := g.New(opts, alice, []game.UserID{bob}, game.StepRNG(seed, 0))
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
	plan := newPlanner(t, game.FirstMoverRandom, seed)
	plan.play(p2, across("a1"))
	plan.play(p1, random())
	plan.play(p2, random())
	plan.sink(plan.state.ToAct()[0], bottomRows())
	var steps []persistedStep
	for _, step := range plan.steps {
		raw, err := json.Marshal(step.Action)
		if err != nil {
			t.Fatal(err)
		}
		steps = append(steps, persistedStep{seat: step.Seat, action: raw})
	}

	live := start()
	for i, step := range steps[:restoreAt] {
		live, _ = apply(live, step, game.Seq(i)+1)
	}
	restored := start()
	for i, step := range steps[:restoreAt] {
		restored, _ = apply(restored, step, game.Seq(i)+1)
	}
	for i, step := range steps[restoreAt:] {
		seq := game.Seq(restoreAt+i) + 1
		for _, seat := range []game.Seat{p1, p2} {
			liveView, _ := live.View(seat)
			restoredView, _ := restored.View(seat)
			if diff := cmp.Diff(string(liveView), string(restoredView)); diff != "" {
				t.Fatalf("seq %d: %s's restored view differs (-live +restored):\n%s", seq, seat, diff)
			}
		}
		var liveEvents, restoredEvents []json.RawMessage
		live, liveEvents = apply(live, step, seq)
		restored, restoredEvents = apply(restored, step, seq)
		if diff := cmp.Diff(liveEvents, restoredEvents); diff != "" {
			t.Fatalf("seq %d: restored events differ (-live +restored):\n%s", seq, diff)
		}
	}
	if got, finished := restored.Outcome(); !finished || got.Reason != ReasonFleetSunk {
		t.Errorf("restored outcome = %+v, %t", got, finished)
	}
}
