package pacman

import (
	"math/rand/v2"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// Points scored.
const (
	DotPoints    = 10
	PelletPoints = 50
	// GhostPoints is the first ghost of a fright's; each next one doubles it.
	GhostPoints = 200
)

// StartingLives is the lives a game starts with, the one in play among them.
const StartingLives = 3

type pacman struct {
	at     grid.Point
	facing grid.Direction
	// turn is the way it turns at the first cell where that way is open; "" for none.
	turn grid.Direction
}

// moved is Pac-Man one tick on: turned if it can, then one cell ahead unless a wall is there.
func (p pacman) moved(m *maze) pacman {
	if p.turn != "" && m.walkable(p.turn.From(p.at)) {
		p.facing, p.turn = p.turn, ""
	}
	if next := wrapped(p.facing.From(p.at)); m.walkable(next) {
		p.at = next
	}
	return p
}

// round counts what a round, from a level's start or a lost life, has seen so far.
type round struct {
	ticks, dots int
	// modeTicks are the ticks spent scattering or chasing, which a fright holds still.
	modeTicks int
	// frightLeft is the ticks the fright has to run; combo the ghosts it has eaten so far.
	frightLeft, combo int
}

type state struct {
	game.Ending
	maze *maze
	// food is the dots and pellets left in maze.
	food   int
	pacman pacman
	ghosts [4]ghost
	round  round
	// tick counts the level's ticks; slower movers move on some of them only.
	tick                int
	score, lives, level int
	paused              bool
}

// newState is level 1's start, paused until the first turn.
func newState() state {
	return state{lives: StartingLives, level: 1, paused: true}.withClassicMaze().newRound()
}

func (s state) withClassicMaze() state {
	s.maze = classic()
	s.food = s.maze.food()
	return s
}

// newRound puts Pac-Man and the ghosts back where they start, the dots as they are.
func (s state) newRound() state {
	s.pacman = pacman{at: pacmanStart(), facing: grid.Left}
	for i, name := range ghostNames() {
		s.ghosts[i] = startingGhost(name)
	}
	s.round = round{}
	return s
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	switch action.Kind {
	case ActionTurn:
		d, err := grid.ParseDirection(action.Direction)
		if err != nil {
			return nil, nil, err
		}
		s.pacman.turn, s.paused = d, false
		return s, nil, nil
	case ActionPause:
		if s.paused {
			return nil, nil, game.Illegalf("already paused")
		}
		s.paused = true
		return s, nil, nil
	}
	return nil, nil, game.UnknownAction(action.Kind)
}

// NextTick implements game.Ticking: one tick of the level's interval, until paused or over.
func (s state) NextTick() (time.Duration, bool) {
	return levelOf(s.level).interval, !s.Over() && !s.paused
}

// Pause implements game.Ticking.
func (s state) Pause() game.State[Action, View] {
	s.paused = true
	return s
}

// Tick implements game.Ticking: Pac-Man moves and eats, then the ghosts move, and either
// catching the other settles it; then the fright and the scatter-chase schedule run on, and
// waiting ghosts leave the house when their time comes. The level clears once Pac-Man has
// met the ghosts on the cell of its last dot and is still free.
func (s state) Tick(rng *rand.Rand) (game.State[Action, View], []game.Event) {
	s.tick++
	s.round.ticks++
	s.pacman = s.pacman.moved(s.maze)
	s = s.fed()
	s, events, caught := s.collided(nil)
	if caught {
		return s, events
	}
	if s.food == 0 {
		return s.cleared(events)
	}
	s = s.ghostsMoved(rng)
	if s, events, caught = s.collided(events); caught {
		return s, events
	}
	return s.timed().released(), events
}

// fed is s after Pac-Man eats what is on its cell; a power pellet frightens the ghosts.
func (s state) fed() state {
	switch s.maze.at(s.pacman.at) {
	case TileDot:
		s.score += DotPoints
	case TilePellet:
		s.score += PelletPoints
		s = s.frightened()
	default:
		return s
	}
	s.maze = s.maze.eaten(s.pacman.at)
	s.food--
	s.round.dots++
	return s
}

func (s state) frightened() state {
	s.round.frightLeft = levelOf(s.level).ticks(levelOf(s.level).fright)
	s.round.combo = 0
	for i, g := range s.ghosts {
		if g.activity != returning {
			g.frightened = true
			s.ghosts[i] = g.reversed()
		}
	}
	return s
}

// collided settles every ghost on Pac-Man's cell: a frightened one is eaten, eyes pass by, and
// any other catches Pac-Man, which ends the round.
func (s state) collided(events []game.Event) (state, []game.Event, bool) {
	for i, g := range s.ghosts {
		if g.at != s.pacman.at || g.activity == returning {
			continue
		}
		if !g.frightened {
			next, caught := s.caughtBy(g)
			return next, append(events, caught), true
		}
		points := GhostPoints << s.round.combo
		s.score += points
		s.round.combo++
		s.ghosts[i] = g.eaten()
		events = append(events, GhostEatenEvent{By: game.SeatSolo, Ghost: g.name, Points: points})
	}
	return s, events, false
}

func (s state) caughtBy(g ghost) (state, game.Event) {
	s.lives--
	event := CaughtEvent{By: game.SeatSolo, Ghost: g.name, LivesLeft: s.lives}
	if s.lives == 0 {
		return s.over(game.LossBy(ReasonGameOver)), event
	}
	s.paused = true
	return s.newRound(), event
}

// cleared is the next level, paused until the next turn, or the game won after the last;
// events are the tick's so far.
func (s state) cleared(events []game.Event) (game.State[Action, View], []game.Event) {
	events = append(events, LevelClearedEvent{By: game.SeatSolo, Level: s.level})
	if s.level == Levels {
		return s.over(game.WinFor(ReasonCleared, game.SeatSolo)), events
	}
	s.level++
	s.tick, s.paused = 0, true
	return s.withClassicMaze().newRound(), events
}

func (s state) over(result game.Result) state {
	s.Ending = s.Concluded(result)
	return s
}

// ghostsMoved moves each ghost whose tick it is toward its target.
func (s state) ghostsMoved(rng *rand.Rand) state {
	moved := s
	for i, g := range s.ghosts {
		if s.moves(g) {
			moved.ghosts[i] = g.stepped(s.maze, s.target(g), rng)
		}
	}
	return moved
}

// moves is whether g moves this tick: eyes always, a roaming ghost but for its rests, and at
// half speed a frightened one, one in the tunnel, or one leaving the house.
func (s state) moves(g ghost) bool {
	switch {
	case g.activity == returning:
		return true
	case g.activity == waiting:
		return false
	case g.activity == leaving || g.frightened || inTunnel(g.at):
		return s.tick%2 == 0
	}
	return s.tick%levelOf(s.level).ghostRest != 0
}

// target is the cell g heads for: home for eyes, its corner when scattering, and when chasing
// Blinky's is Pac-Man, Pinky's 4 cells ahead of it, Inky's Blinky's offset from 2 cells ahead
// of it, doubled, and Clyde's Pac-Man while more than 8 cells away, else its corner.
func (s state) target(g ghost) grid.Point {
	pac := s.pacman
	switch {
	case g.activity == returning:
		return houseExit()
	case s.roamingMode() == ModeScatter:
		return homeOf(g.name).corner
	}
	switch g.name {
	case Pinky:
		return ahead(pac.at, pac.facing, 4)
	case Inky:
		pivot, blinky := ahead(pac.at, pac.facing, 2), s.ghosts[0].at
		return grid.Point{Row: 2*pivot.Row - blinky.Row, Col: 2*pivot.Col - blinky.Col}
	case Clyde:
		const shyDistance = 8
		if distance2(g.at, pac.at) <= shyDistance*shyDistance {
			return homeOf(Clyde).corner
		}
	}
	return pac.at
}

func (s state) roamingMode() GhostMode { return levelOf(s.level).modeAfter(s.round.modeTicks) }

// timed runs the fright down, or else the schedule on; ghosts in the maze turn back each time the
// schedule changes mode.
func (s state) timed() state {
	if s.round.frightLeft > 0 {
		s.round.frightLeft--
		if s.round.frightLeft == 0 {
			for i := range s.ghosts {
				s.ghosts[i].frightened = false
			}
		}
		return s
	}
	before := s.roamingMode()
	s.round.modeTicks++
	if s.roamingMode() != before {
		for i, g := range s.ghosts {
			s.ghosts[i] = g.reversed()
		}
	}
	return s
}

// released sends waiting ghosts out of the house once the round has seen their dots or time.
func (s state) released() state {
	l := levelOf(s.level)
	for i, g := range s.ghosts {
		h := homeOf(g.name)
		if g.activity == waiting && (s.round.dots >= h.releaseDots || s.round.ticks >= l.ticks(time.Duration(h.releaseSeconds)*time.Second)) {
			s.ghosts[i].activity = leaving
		}
	}
	return s
}

func (s state) View(game.Seat) View {
	ghosts := make([]GhostView, len(s.ghosts))
	for i, g := range s.ghosts {
		ghosts[i] = g.view(s.roamingMode())
	}
	var turn *grid.Direction
	if s.pacman.turn != "" {
		turn = new(s.pacman.turn)
	}
	return View{
		Maze:        s.maze.lines(),
		Pacman:      PacmanView{Actor: actorAt(s.pacman.at, s.pacman.facing), NextTurn: turn},
		Ghosts:      ghosts,
		Score:       s.score,
		Lives:       s.lives,
		Level:       s.level,
		Levels:      Levels,
		FoodLeft:    s.food,
		FrightTicks: s.round.frightLeft,
		Paused:      s.paused,
		CanTurn:     !s.Over(),
		CanPause:    !s.Over() && !s.paused,
	}
}

func (s state) ToAct() []game.Seat { return s.ToActOf(game.SeatSolo) }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining([]game.Seat{game.SeatSolo}, seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game, and no action is open.
func (s state) Conclude(result game.Result) game.State[Action, View] { return s.over(result) }
