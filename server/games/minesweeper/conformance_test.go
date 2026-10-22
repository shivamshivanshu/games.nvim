package minesweeper

import (
	"testing"

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
	return &planner{t: t, seed: seed, state: gametest.NewSolo[state](t, Rules{}, Opts{Difficulty: d}, solo, seed)}
}

func (p *planner) play(action Action) {
	p.t.Helper()
	next, _, err := p.state.Apply(game.SeatSolo, action, game.StepRNG(p.seed, game.Seq(p.applied()+1)))
	if err != nil {
		p.t.Fatalf("plan %+v: %v", action, err)
	}
	p.state = next.(state)
	p.steps = append(p.steps, gametest.Step[Action]{Seat: game.SeatSolo, Action: action})
}

func (p *planner) refuse(action Action) {
	p.steps = append(p.steps, gametest.Step[Action]{Seat: game.SeatSolo, Action: action, WantErr: true})
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
	cleared := game.WinFor(ReasonCleared, game.SeatSolo)
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
	mine := game.LossBy(ReasonMine)
	scripts = append(scripts, lost.script("reveals a mine", &mine))
	unfinished := newPlanner(t, DifficultyMedium, 9)
	unfinished.play(reveal("h8"))
	unfinished.play(flag("a1"))
	scripts = append(scripts, unfinished.script("unfinished", nil))
	gametest.Run(t, Rules{}, scripts)
}
