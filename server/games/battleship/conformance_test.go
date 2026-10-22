package battleship

import (
	"slices"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
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
	st := gametest.NewState[game.State[Action, View]](t, Rules{}, Opts{First: first}, []game.UserID{alice, bob}, seed)
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
		p.state, _ = gametest.MustApply(p.t, Rules{}.Info(), p.state, seat, action, game.StepRNG(p.seed, p.applied()+1))
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
	opponent := sides.Other(shooter)
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
		Name: name, Opts: Opts{First: first},
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

func scripts(t *testing.T) []gametest.Script[Opts, Action] {
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

	return []gametest.Script[Opts, Action]{
		fullGame(t, game.FirstMoverMe, 1).script("p1 sinks a fleet placed by hand", game.FirstMoverMe, p1Wins),
		bothRandom.script("both fill at random, p2 sinks p1", game.FirstMoverOpponent, p2Wins),
		setup.script("still placing", game.FirstMoverRandom, nil),
	}
}

func TestConformance(t *testing.T) {
	gametest.Run(t, Rules{}, scripts(t))
}

// TestUnhitShipsReachNoOtherSeat moves a ship no shot has touched and
// requires the other seat to be sent the same, unless it fires at a square
// the move changed or the game ends and shows both fleets.
func TestUnhitShipsReachNoOtherSeat(t *testing.T) {
	hidden := gametest.Hidden[Action, View]{Perturb: movedUnhitShip, Reveals: firedAtTheMove}
	for _, script := range scripts(t) {
		t.Run(script.Name, func(t *testing.T) { gametest.NonInterference(t, Rules{}, script, hidden) })
	}
}

// movedUnhitShip is s with owner's first ship that no shot has touched moved
// to the first other berth no shot has touched; unchanged when there is none.
func movedUnhitShip(s game.State[Action, View], owner game.Seat) (game.State[Action, View], []game.Seat) {
	st := s.(state)
	own := st.waters[owner]
	shot := func(p grid.Point) bool { return own.shot[index(p)] }
	for i, ship := range own.ships {
		if slices.ContainsFunc(ship.points, shot) {
			continue
		}
		others := waters{ships: slices.Delete(slices.Clone(own.ships), i, i+1), shot: own.shot}
		for _, o := range orientations() {
			for _, anchor := range others.anchors(ship.spec.Size, o) {
				points := footprint(anchor, ship.spec.Size, o)
				if slices.Equal(points, ship.points) || slices.ContainsFunc(points, shot) {
					continue
				}
				ships := slices.Clone(own.ships)
				ships[i] = placedShip{spec: ship.spec, points: points}
				st.waters = xslices.With(st.waters, owner, waters{ships: ships, shot: own.shot})
				return st, []game.Seat{owner}
			}
		}
	}
	return s, nil
}

// firedAtTheMove tells the shooter of a square the move changed, and both
// seats of the shot that ends the game.
func firedAtTheMove(s game.State[Action, View], step gametest.Step[Action], owners []game.Seat) []game.Seat {
	if step.Action.Kind != ActionFire {
		return nil
	}
	next, _, err := s.Apply(step.Seat, step.Action, nil)
	if err != nil {
		return nil
	}
	if _, over := next.Outcome(); over {
		return sides.Seats()
	}
	owner := owners[0]
	p, _ := coords().Parse(step.Action.Cell)
	moved, _ := movedUnhitShip(s, owner)
	_, wasShip := s.(state).waters[owner].shipAt(p)
	_, isShip := moved.(state).waters[owner].shipAt(p)
	if step.Seat != owner && wasShip != isShip {
		return []game.Seat{step.Seat}
	}
	return nil
}
