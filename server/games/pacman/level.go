package pacman

import "time"

// Levels is how many levels there are: clearing the last wins the game.
const Levels = 3

// level is how one level plays. Times are in real time and become ticks of
// the level's interval, so a faster level keeps the same schedule in seconds.
type level struct {
	interval time.Duration
	fright   time.Duration
	// ghostRest makes roaming ghosts slower than Pac-Man: they rest one tick in every ghostRest.
	ghostRest int
	// phases are the scatter and chase spells of a round, in order; chase goes on after the last.
	phases []phase
}

type phase struct {
	mode  GhostMode
	lasts time.Duration
}

var levels = [Levels]level{
	{interval: 150 * time.Millisecond, fright: 6 * time.Second, ghostRest: 5, phases: phases(7, 20, 7, 20, 5, 20, 5)},
	{interval: 135 * time.Millisecond, fright: 4 * time.Second, ghostRest: 8, phases: phases(7, 20, 7, 20, 5)},
	{interval: 120 * time.Millisecond, fright: 2 * time.Second, ghostRest: 12, phases: phases(5, 20, 5, 20, 5)},
}

// levelOf is level n, counted from 1; a level outside 1 to Levels plays as the last.
func levelOf(n int) level {
	if n < 1 || n > Levels {
		return levels[Levels-1]
	}
	return levels[n-1]
}

// phases alternate scatter and chase, starting with scatter, for the given seconds each.
func phases(seconds ...int) []phase {
	out := make([]phase, len(seconds))
	for i, s := range seconds {
		mode := ModeScatter
		if i%2 == 1 {
			mode = ModeChase
		}
		out[i] = phase{mode: mode, lasts: time.Duration(s) * time.Second}
	}
	return out
}

func (l level) ticks(d time.Duration) int { return int(d / l.interval) }

// modeAfter is the ghosts' mode once a round has spent ticks scattering and chasing.
func (l level) modeAfter(ticks int) GhostMode {
	for _, p := range l.phases {
		ticks -= l.ticks(p.lasts)
		if ticks < 0 {
			return p.mode
		}
	}
	return ModeChase
}
