package minesweeper

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

func difficulties() []Difficulty {
	return []Difficulty{DifficultyEasy, DifficultyMedium, DifficultyHard}
}

// planner plays a game step by step, so a script knows the mines its first reveal places.
type planner struct {
	t     *testing.T
	seed  game.Seed
	state state
	steps []gametest.Step[Action]
}

func newPlanner(t *testing.T, d Difficulty, seed game.Seed) *planner {
	t.Helper()
	st, _, err := Rules{}.New(Opts{Difficulty: d}, solo, nil, game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	return &planner{t: t, seed: seed, state: st.(state)}
}

func (p *planner) play(action Action) {
	p.t.Helper()
	next, _, err := p.state.Apply(SeatSolver, action, game.StepRNG(p.seed, game.Seq(p.applied()+1)))
	if err != nil {
		p.t.Fatalf("plan %+v: %v", action, err)
	}
	p.state = next.(state)
	p.steps = append(p.steps, gametest.Step[Action]{Seat: SeatSolver, Action: action})
}

func (p *planner) refuse(action Action) {
	p.steps = append(p.steps, gametest.Step[Action]{Seat: SeatSolver, Action: action, WantErr: true})
}

func (p *planner) applied() int {
	n := 0
	for _, step := range p.steps {
		if !step.WantErr {
			n++
		}
	}
	return n
}

func (p *planner) at(cell int) string { return address(p.state.lvl.point(cell)) }

// chordOnce flags the mines around the first number that has a safe hidden neighbour, then chords it.
func (p *planner) chordOnce() {
	p.t.Helper()
	s := p.state
	for cell := range s.lvl.cells() {
		if !s.open[cell] || s.minesAround(cell) == 0 || !p.hasSafeCovered(cell) {
			continue
		}
		for _, n := range s.lvl.neighbours(cell) {
			if s.mines[n] && !s.flags[n] {
				p.play(flag(p.at(n)))
			}
		}
		p.play(chord(p.at(cell)))
		return
	}
	p.t.Fatal("no number to chord after the first reveal")
}

func (p *planner) hasSafeCovered(cell int) bool {
	for _, n := range p.state.coveredAround(cell) {
		if !p.state.mines[n] {
			return true
		}
	}
	return false
}

func (p *planner) clearTheRest() {
	for cell := range p.state.lvl.cells() {
		if !p.state.mines[cell] && !p.state.open[cell] {
			p.play(reveal(p.at(cell)))
		}
	}
}

func (p *planner) firstMine() int {
	for cell, mine := range p.state.mines {
		if mine {
			return cell
		}
	}
	p.t.Fatal("no mine placed")
	return noCell
}

func (p *planner) script(name string, want *game.Result) gametest.Script[Opts, Action] {
	lvl := p.state.lvl
	opts := Opts{}
	for _, d := range difficulties() {
		if l, _ := d.level(); l == lvl {
			opts.Difficulty = d
		}
	}
	return gametest.Script[Opts, Action]{
		Name: name, Opts: opts, Challenger: solo, Seed: p.seed, Steps: p.steps, WantResult: want,
		Secrets: []string{string(CellMine), string(CellFlaggedMine), string(CellExploded), string(CellWrongFlag)},
	}
}

const conformanceSeed game.Seed = 7

// clearingScript refuses some actions, flags and unflags, opens the centre, chords once, then
// opens every safe cell left.
func clearingScript(t *testing.T, d Difficulty, seed game.Seed) gametest.Script[Opts, Action] {
	p := newPlanner(t, d, seed)
	lvl := p.state.lvl
	centre := p.at(lvl.cells() / 2)
	p.refuse(chord(centre))
	p.refuse(reveal("q1"))
	p.play(flag("a1"))
	p.refuse(reveal("a1"))
	p.play(flag("a1"))
	p.play(reveal(centre))
	p.refuse(reveal(centre))
	p.chordOnce()
	p.clearTheRest()
	p.refuse(reveal("a1"))
	cleared := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonCleared}
	return p.script("clears "+string(d), &cleared)
}

func TestConformance(t *testing.T) {
	var scripts []gametest.Script[Opts, Action]
	for _, d := range difficulties() {
		scripts = append(scripts, clearingScript(t, d, conformanceSeed))
	}
	lost := newPlanner(t, DifficultyEasy, 8)
	lost.play(reveal("e5"))
	lost.play(reveal(lost.at(lost.firstMine())))
	mine := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{}, Reason: ReasonMine}
	scripts = append(scripts, lost.script("reveals a mine", &mine))
	unfinished := newPlanner(t, DifficultyMedium, 9)
	unfinished.play(reveal("h8"))
	unfinished.play(flag("a1"))
	scripts = append(scripts, unfinished.script("unfinished", nil))
	gametest.Run(t, Rules{}, scripts)
}

type persistedStep struct {
	seat   game.Seat
	action json.RawMessage
}

func TestRestoreReplaysPersistedStepsAndPlaysOnIdentically(t *testing.T) {
	const seed game.Seed = 13
	const restoreAt = 1
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(json.RawMessage(`{"difficulty":"medium"}`))
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
	for _, step := range clearingScript(t, DifficultyMedium, seed).Steps {
		if step.WantErr {
			continue
		}
		raw, err := json.Marshal(step.Action)
		if err != nil {
			t.Fatal(err)
		}
		plan = append(plan, persistedStep{seat: step.Seat, action: raw})
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
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonCleared}
	if got, finished := restored.Outcome(); !finished || !cmp.Equal(got, want) {
		t.Errorf("restored outcome = %+v, %t; want %+v", got, finished, want)
	}
}
