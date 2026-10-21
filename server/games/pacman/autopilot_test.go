package pacman

import (
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// towardFood is the first way along a shortest path from Pac-Man to the nearest food that
// passes no ghost a frightened one aside, or Pac-Man's facing when there is none.
func towardFood(s state) grid.Direction {
	blocked := map[grid.Point]bool{}
	for _, g := range s.ghosts {
		if !g.frightened && g.activity != returning {
			blocked[g.at] = true
		}
	}
	first := map[grid.Point]grid.Direction{s.pacman.at: ""}
	queue := []grid.Point{s.pacman.at}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if t := s.maze.at(p); p != s.pacman.at && (t == TileDot || t == TilePellet) {
			return first[p]
		}
		for _, d := range grid.Directions() {
			n := wrapped(d.From(p))
			if _, seen := first[n]; seen || blocked[n] || !s.maze.walkable(n) {
				continue
			}
			first[n] = first[p]
			if p == s.pacman.at {
				first[n] = d
			}
			queue = append(queue, n)
		}
	}
	return s.pacman.facing
}

// autoplay is the steps of a game the autopilot plays from seed's start: a turn toward food
// whenever its way changes or the game is paused, else a tick, until the game ends or limit
// steps are made. It returns the steps and the state they reach.
func autoplay(t *testing.T, seed game.Seed, limit int) ([]gametest.Step[Action], state) {
	t.Helper()
	s := newState()
	var steps []gametest.Step[Action]
	for !s.Over() && len(steps) < limit {
		seq := game.Seq(len(steps) + 1)
		if way := towardFood(s); s.paused || way != s.pacman.facing && way != s.pacman.turn {
			steps = append(steps, gametest.Step[Action]{Seat: SeatPlayer, Action: turn(way)})
			next, _, err := s.Apply(SeatPlayer, turn(way), game.StepRNG(seed, seq))
			if err != nil {
				t.Fatal(err)
			}
			s = next.(state)
			continue
		}
		steps = append(steps, gametest.Step[Action]{Kind: gametest.StepTick})
		next, _ := s.Tick(game.StepRNG(seed, seq))
		s = next.(state)
	}
	return steps, s
}

func turn(d grid.Direction) Action { return Action{Kind: ActionTurn, Direction: string(d)} }
