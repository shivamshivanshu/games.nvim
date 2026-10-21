package pacman

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// ghostNames lists the ghosts in View order.
func ghostNames() [4]GhostName { return [4]GhostName{Blinky, Pinky, Inky, Clyde} }

// activity is where a ghost is in its round trip through the house.
type activity int

const (
	// waiting in the house until its release.
	waiting activity = iota
	// leaving the house: across to the door's column, then up through it.
	leaving
	// roaming the maze, scattering or chasing, or frightened.
	roaming
	// returning as eyes to the house once eaten: to houseExit, then down to houseCentre.
	returning
)

type ghost struct {
	name       GhostName
	at         grid.Point
	facing     grid.Direction
	activity   activity
	frightened bool
}

// home is where a ghost starts each round, and the corner it scatters to.
type home struct {
	start    grid.Point
	activity activity
	corner   grid.Point
	// releaseDots and releaseSeconds release a waiting ghost once the round has seen that many
	// dots eaten or that much time, whichever comes first.
	releaseDots, releaseSeconds int
}

func homeOf(name GhostName) home {
	switch name {
	case Blinky:
		return home{start: houseExit(), activity: roaming, corner: grid.Point{Row: -3, Col: Cols - 3}}
	case Pinky:
		return home{start: houseCentre(), corner: grid.Point{Row: -3, Col: 2}}
	case Inky:
		return home{start: grid.Point{Row: 14, Col: 11}, corner: grid.Point{Row: Rows, Col: Cols - 1}, releaseDots: 30, releaseSeconds: 4}
	}
	return home{start: grid.Point{Row: 14, Col: 16}, corner: grid.Point{Row: Rows, Col: 0}, releaseDots: 60, releaseSeconds: 8}
}

func startingGhost(name GhostName) ghost {
	h := homeOf(name)
	return ghost{name: name, at: h.start, facing: grid.Left, activity: h.activity}
}

func (g ghost) mode(roamingMode GhostMode) GhostMode {
	switch {
	case g.activity == returning:
		return ModeEyes
	case g.frightened:
		return ModeFrightened
	case g.activity == roaming:
		return roamingMode
	}
	return ModeHouse
}

func (g ghost) view(roamingMode GhostMode) GhostView {
	return GhostView{Actor: actorAt(g.at, g.facing), Name: g.name, Mode: g.mode(roamingMode)}
}

func (g ghost) reversed() ghost {
	if g.activity == roaming {
		g.facing = g.facing.Opposite()
	}
	return g
}

// eaten is the ghost's eyes, on the way home.
func (g ghost) eaten() ghost {
	g.activity, g.frightened = returning, false
	return g
}

// stepped is the ghost one cell on toward target, or at random when frightened in the maze.
func (g ghost) stepped(m *maze, target grid.Point, rng *rand.Rand) ghost {
	switch {
	case g.activity == leaving:
		return g.leftHouse()
	case g.activity == returning && g.enteringHouse():
		g.facing = grid.Down
	default:
		g.facing = g.choice(m, target, rng)
	}
	g.at = wrapped(g.facing.From(g.at))
	if g.activity == returning && g.at == houseCentre() {
		g.activity = leaving
	}
	return g
}

// enteringHouse is whether returning eyes are at the exit or between it and the house's centre.
func (g ghost) enteringHouse() bool {
	exit, centre := houseExit(), houseCentre()
	return g.at.Col == exit.Col && g.at.Row >= exit.Row && g.at.Row < centre.Row
}

func (g ghost) leftHouse() ghost {
	exit := houseExit()
	switch {
	case g.at.Col < exit.Col:
		g.facing = grid.Right
	case g.at.Col > exit.Col:
		g.facing = grid.Left
	default:
		g.facing = grid.Up
	}
	g.at = g.facing.From(g.at)
	if g.at == exit {
		g.activity, g.facing = roaming, grid.Left
	}
	return g
}

// choice is the way a ghost in the maze takes from its cell: never back the way it came unless
// that is the only way, frightened at random, else the way whose next cell is nearest target,
// ties broken up, left, down, right as in the arcade.
func (g ghost) choice(m *maze, target grid.Point, rng *rand.Rand) grid.Direction {
	var open []grid.Direction
	for _, d := range []grid.Direction{grid.Up, grid.Left, grid.Down, grid.Right} {
		if d != g.facing.Opposite() && m.walkable(d.From(g.at)) {
			open = append(open, d)
		}
	}
	switch {
	case len(open) == 0:
		return g.facing.Opposite()
	case g.frightened:
		return open[rng.IntN(len(open))]
	}
	best := open[0]
	for _, d := range open[1:] {
		if distance2(wrapped(d.From(g.at)), target) < distance2(wrapped(best.From(g.at)), target) {
			best = d
		}
	}
	return best
}

// distance2 is the squared straight-line distance between a and b, in cells.
func distance2(a, b grid.Point) int {
	dRow, dCol := a.Row-b.Row, a.Col-b.Col
	return dRow*dRow + dCol*dCol
}

// ahead is n cells from p toward d, on or off the maze.
func ahead(p grid.Point, d grid.Direction, n int) grid.Point {
	for range n {
		p = d.From(p)
	}
	return p
}

func actorAt(p grid.Point, facing grid.Direction) Actor {
	return Actor{Row: p.Row + 1, Col: p.Col + 1, Facing: facing}
}
