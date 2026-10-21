package pacman

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

const player game.UserID = "alice"

func pt(row, col int) grid.Point { return grid.Point{Row: row, Col: col} }

// running is the start with its pause lifted and every ghost but Blinky held in the house.
func running() state {
	s := newState()
	s.paused = false
	return s
}

// tickOnce ticks s with the rng of seed's step seq, as the core numbers steps.
func tickOnce(s state, seq game.Seq) (state, []game.Event) {
	next, events := s.Tick(game.StepRNG(1, seq))
	return next.(state), events
}

func ticked(s state, n int) state {
	for i := range n {
		s, _ = tickOnce(s, game.Seq(i+1))
	}
	return s
}

// alone is s with every ghost shut in the house, out of the way.
func alone(s state) state {
	for i := range s.ghosts {
		s.ghosts[i] = ghost{name: s.ghosts[i].name, at: houseCentre(), facing: grid.Up, activity: waiting}
	}
	s.round.ticks, s.round.dots = -1_000_000, -1_000_000
	return s
}

// emptied is s's maze with no food but at the given cells.
func emptied(s state, food ...grid.Point) state {
	for r := range Rows {
		for c := range Cols {
			if t := s.maze[r][c]; t == TileDot || t == TilePellet {
				s.maze[r][c] = TileOpen
			}
		}
	}
	for _, p := range food {
		s.maze[p.Row][p.Col] = TileDot
	}
	return s
}

func TestClassicMazeIsTheArcadeMaze(t *testing.T) {
	m := classic()
	counts := map[Tile]int{}
	for r := range Rows {
		for c := range Cols {
			counts[m[r][c]]++
			if mirror := m[r][Cols-1-c]; mirror != m[r][c] {
				t.Errorf("(%d,%d) is %q but its mirror is %q", r, c, m[r][c], mirror)
			}
		}
	}
	if counts[TileDot] != 240 || counts[TilePellet] != 4 || counts[TileDoor] != 2 {
		t.Errorf("dots %d, pellets %d, door %d; want 240, 4 and 2", counts[TileDot], counts[TilePellet], counts[TileDoor])
	}
	for _, p := range []grid.Point{pacmanStart(), houseExit(), houseCentre()} {
		if m.at(p) == TileWall {
			t.Errorf("%v is a wall", p)
		}
	}
	if m.at(pt(12, 13)) != TileDoor || m.walkable(pt(12, 13)) {
		t.Error("the door above the house's centre is not a door that Pac-Man is kept from")
	}
	if !m.walkable(pt(14, -1)) || wrapped(pt(14, -1)) != pt(14, Cols-1) || m.walkable(pt(-1, 13)) {
		t.Error("the tunnel does not wrap, or the maze is open off its top")
	}
}

func TestPacmanTurnsAtTheFirstOpenCellAndStopsAtWalls(t *testing.T) {
	tests := []struct {
		name         string
		at           grid.Point
		facing, turn grid.Direction
		wantAt       grid.Point
		wantFacing   grid.Direction
		wantTurn     grid.Direction
	}{
		{"a blocked turn waits while it goes on", pt(23, 13), grid.Left, grid.Up, pt(23, 12), grid.Left, grid.Up},
		{"an open turn is taken", pt(23, 12), grid.Left, grid.Up, pt(22, 12), grid.Up, ""},
		{"turning back is always open", pt(23, 12), grid.Left, grid.Right, pt(23, 13), grid.Right, ""},
		{"a wall stops it", pt(29, 1), grid.Left, "", pt(29, 1), grid.Left, ""},
		{"the tunnel wraps left", pt(14, 0), grid.Left, "", pt(14, Cols-1), grid.Left, ""},
		{"the tunnel wraps right", pt(14, Cols-1), grid.Right, "", pt(14, 0), grid.Right, ""},
		{"the door is shut to it", pt(11, 13), grid.Down, "", pt(11, 13), grid.Down, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pacman{at: tt.at, facing: tt.facing, turn: tt.turn}.moved(classic())
			if want := (pacman{at: tt.wantAt, facing: tt.wantFacing, turn: tt.wantTurn}); got != want {
				t.Errorf("moved = %+v, want %+v", got, want)
			}
		})
	}
}

func TestEatingScoresAndAPelletFrightensTheGhosts(t *testing.T) {
	s := running()
	s.pacman = pacman{at: pt(1, 2), facing: grid.Left}
	s, _ = tickOnce(s, 1)
	if s.score != DotPoints || s.maze.at(pt(1, 1)) != TileOpen || s.round.dots != 1 {
		t.Fatalf("after a dot: score %d, cell %q, round dots %d", s.score, s.maze.at(pt(1, 1)), s.round.dots)
	}

	s = running()
	s.pacman = pacman{at: pt(4, 1), facing: grid.Up}
	blinky := s.ghosts[0]
	s, _ = tickOnce(s, 1)
	fright := levelOf(1).ticks(levelOf(1).fright)
	if s.score != PelletPoints || s.round.frightLeft != fright-1 {
		t.Fatalf("after a pellet: score %d, fright left %d, want %d and %d", s.score, s.round.frightLeft, PelletPoints, fright-1)
	}
	for _, g := range s.ghosts {
		if !g.frightened {
			t.Errorf("%s is not frightened", g.name)
		}
	}
	if s.ghosts[0].facing == blinky.facing && s.ghosts[0].at == blinky.at {
		t.Error("Blinky did not turn back when frightened")
	}
	s = ticked(s, fright-1)
	if slices.ContainsFunc(s.ghosts[:], func(g ghost) bool { return g.frightened }) || s.round.frightLeft != 0 {
		t.Errorf("still frightened after the fright: %+v", s.ghosts)
	}
}

func TestEatingFrightenedGhostsDoublesEachTime(t *testing.T) {
	s := alone(running())
	s.pacman = pacman{at: pt(5, 5), facing: grid.Right}
	s.maze = s.maze.eaten(pt(5, 6))
	for i := range s.ghosts {
		s.ghosts[i] = ghost{name: s.ghosts[i].name, at: pt(5, 6), facing: grid.Left, activity: roaming, frightened: true}
	}
	s.round.combo = 0
	s, events := tickOnce(s, 1)
	want := []game.Event{
		GhostEatenEvent{By: SeatPlayer, Ghost: Blinky, Points: 200},
		GhostEatenEvent{By: SeatPlayer, Ghost: Pinky, Points: 400},
		GhostEatenEvent{By: SeatPlayer, Ghost: Inky, Points: 800},
		GhostEatenEvent{By: SeatPlayer, Ghost: Clyde, Points: 1600},
	}
	if diff := cmp.Diff(want, events); diff != "" || s.score != 3000 {
		t.Fatalf("score %d; events (-want +got):\n%s", s.score, diff)
	}
	for _, g := range s.View(SeatPlayer).Ghosts {
		if g.Mode != ModeEyes {
			t.Errorf("%s is %s after being eaten, want eyes", g.Name, g.Mode)
		}
	}
}

// reachable is every cell Pac-Man can walk to from its start.
func reachable(m maze) []grid.Point {
	seen := map[grid.Point]bool{pacmanStart(): true}
	out := []grid.Point{pacmanStart()}
	for i := 0; i < len(out); i++ {
		for _, d := range grid.Directions() {
			if n := wrapped(d.From(out[i])); m.walkable(n) && !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out
}

func TestEyesFindTheHouseFromAnywhereAndLeaveAgain(t *testing.T) {
	m := classic()
	for _, from := range reachable(m) {
		for _, facing := range grid.Directions() {
			g := ghost{name: Blinky, at: from, facing: facing, activity: returning}
			steps := 0
			for ; g.activity != roaming && steps < 200; steps++ {
				g = g.stepped(m, houseExit(), nil)
			}
			if g.activity != roaming || g.at != houseExit() {
				t.Fatalf("eyes from %v facing %s: after %d steps %+v, want out of the house", from, facing, steps, g)
			}
		}
	}
}

func TestGhostsNeverTurnBackAndBreakTiesUpLeftDownRight(t *testing.T) {
	m := classic()
	tests := []struct {
		name   string
		g      ghost
		target grid.Point
		want   grid.Direction
	}{
		{"nearest to the target", ghost{at: pt(5, 6), facing: grid.Right}, pt(30, 6), grid.Down},
		{"never back, even toward the target", ghost{at: pt(5, 6), facing: grid.Right}, pt(5, 0), grid.Up},
		{"a tie goes up first", ghost{at: pt(5, 6), facing: grid.Right}, pt(5, 6), grid.Up},
		{"a corridor goes on", ghost{at: pt(1, 3), facing: grid.Right}, pt(1, 0), grid.Right},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.g.choice(m, tt.target, nil); got != tt.want {
				t.Errorf("choice = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestFrightenedGhostsWanderFromTheTicksRNG(t *testing.T) {
	g := ghost{at: pt(5, 6), facing: grid.Right, activity: roaming, frightened: true}
	seen := map[grid.Direction]bool{}
	for seq := range game.Seq(40) {
		d := g.choice(classic(), pt(0, 0), game.StepRNG(1, seq))
		if d == grid.Left {
			t.Fatalf("seq %d: a frightened ghost turned back", seq)
		}
		if again := g.choice(classic(), pt(0, 0), game.StepRNG(1, seq)); again != d {
			t.Fatalf("seq %d: %s then %s from the same rng", seq, d, again)
		}
		seen[d] = true
	}
	if len(seen) != 3 {
		t.Errorf("over 40 seeds a frightened ghost at a junction took %v, want each of its 3 ways", seen)
	}
}

func TestEachGhostHuntsItsOwnWay(t *testing.T) {
	s := running()
	s.round.modeTicks = levelOf(1).ticks(7 * time.Second)
	s.pacman = pacman{at: pt(23, 6), facing: grid.Up}
	s.ghosts[0].at = pt(20, 3)
	near, far := pt(26, 6), pt(5, 21)
	tests := []struct {
		name GhostName
		at   grid.Point
		want grid.Point
	}{
		{Blinky, pt(20, 3), pt(23, 6)},
		{Pinky, pt(1, 1), pt(19, 6)},
		{Inky, pt(1, 1), pt(22, 9)},
		{Clyde, far, pt(23, 6)},
		{Clyde, near, homeOf(Clyde).corner},
	}
	for _, tt := range tests {
		g := ghost{name: tt.name, at: tt.at, activity: roaming}
		if got := s.target(g); got != tt.want {
			t.Errorf("%s at %v chasing: target %v, want %v", tt.name, tt.at, got, tt.want)
		}
	}
	s.round.modeTicks = 0
	for _, name := range ghostNames() {
		if got := s.target(ghost{name: name, activity: roaming}); got != homeOf(name).corner {
			t.Errorf("%s scattering: target %v, want its corner %v", name, got, homeOf(name).corner)
		}
	}
	if got := s.target(ghost{name: Pinky, activity: returning}); got != houseExit() {
		t.Errorf("eyes target %v, want the house's exit", got)
	}
}

func TestScatterAndChaseFollowTheLevelsScheduleAndFrightHoldsIt(t *testing.T) {
	l := levelOf(1)
	seconds := func(n int) int { return l.ticks(time.Duration(n) * time.Second) }
	for _, tc := range []struct {
		ticks int
		want  GhostMode
	}{
		{0, ModeScatter}, {seconds(7) - 1, ModeScatter}, {seconds(7), ModeChase},
		{seconds(27), ModeScatter}, {seconds(84), ModeChase}, {seconds(10_000), ModeChase},
	} {
		if got := l.modeAfter(tc.ticks); got != tc.want {
			t.Errorf("after %d ticks: %s, want %s", tc.ticks, got, tc.want)
		}
	}

	s := running()
	s.round.modeTicks = seconds(7) - 1
	facing := s.ghosts[0].facing
	if s = s.timed(); s.roamingMode() != ModeChase || s.ghosts[0].facing != facing.Opposite() {
		t.Errorf("at the switch to chase: %s, Blinky facing %s; want chase and turned back", s.roamingMode(), s.ghosts[0].facing)
	}
	s.round.frightLeft = 2
	if held := s.timed(); held.round.modeTicks != s.round.modeTicks {
		t.Errorf("the schedule ran on during a fright: %d -> %d", s.round.modeTicks, held.round.modeTicks)
	}
}

func TestGhostsLeaveTheHouseByDotsOrTime(t *testing.T) {
	s := running()
	if s.released().ghosts[1].activity != leaving {
		t.Error("Pinky did not leave at once")
	}
	for _, tc := range []struct {
		name       string
		dots, secs int
		want       [4]activity
	}{
		{"at the start", 0, 0, [4]activity{roaming, leaving, waiting, waiting}},
		{"after 30 dots", 30, 0, [4]activity{roaming, leaving, leaving, waiting}},
		{"after 60 dots", 60, 0, [4]activity{roaming, leaving, leaving, leaving}},
		{"after 4 seconds", 0, 4, [4]activity{roaming, leaving, leaving, waiting}},
		{"after 8 seconds", 0, 8, [4]activity{roaming, leaving, leaving, leaving}},
	} {
		s := running()
		s.round.dots, s.round.ticks = tc.dots, levelOf(1).ticks(time.Duration(tc.secs)*time.Second)
		var got [4]activity
		for i, g := range s.released().ghosts {
			got[i] = g.activity
		}
		if got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
	out := ticked(running(), 20)
	if p := out.ghosts[1]; p.activity != roaming || p.at.Row > houseExit().Row {
		t.Errorf("Pinky after 20 ticks: %+v, want out in the maze", p)
	}
}

func TestACaughtPacmanLosesALifeAndTheRoundRestartsPaused(t *testing.T) {
	s := running()
	s.pacman = pacman{at: pt(5, 5), facing: grid.Right}
	s.ghosts[0] = ghost{name: Blinky, at: pt(5, 6), facing: grid.Left, activity: roaming}
	s.maze = s.maze.eaten(pt(5, 6))
	s.score = 120
	s, events := tickOnce(s, 1)
	if diff := cmp.Diff([]game.Event{CaughtEvent{By: SeatPlayer, Ghost: Blinky, LivesLeft: 2}}, events); diff != "" {
		t.Fatalf("events (-want +got):\n%s", diff)
	}
	if s.lives != 2 || !s.paused || s.pacman.at != pacmanStart() || s.ghosts[0].at != houseExit() || s.score != 120 || s.finished {
		t.Fatalf("after being caught: %+v", s)
	}
	if s.maze.at(pt(5, 6)) != TileOpen {
		t.Error("the eaten dots came back")
	}

	s.lives, s.paused = 1, false
	s.pacman = pacman{at: pt(5, 5), facing: grid.Right}
	s.ghosts[0] = ghost{name: Blinky, at: pt(5, 7), facing: grid.Left, activity: roaming}
	s.tick = 1
	s, _ = tickOnce(s, 2)
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{}, Reason: ReasonGameOver}
	if got, done := s.Outcome(); !done || !cmp.Equal(got, want) || s.lives != 0 {
		t.Fatalf("caught on the last life: %+v, %t, lives %d; want %+v", got, done, s.lives, want)
	}
	if _, due := s.NextTick(); due {
		t.Error("a lost game wants a tick")
	}
}

func TestClearingALevelSpeedsUpAndTheLastWins(t *testing.T) {
	s := alone(running())
	s.pacman = pacman{at: pt(1, 2), facing: grid.Left}
	s = emptied(s, pt(1, 1))
	s.score = 990
	next, events := s.Tick(game.StepRNG(1, 1))
	s = next.(state)
	if diff := cmp.Diff([]game.Event{LevelClearedEvent{By: SeatPlayer, Level: 1}}, events); diff != "" {
		t.Fatalf("events (-want +got):\n%s", diff)
	}
	if s.level != 2 || !s.paused || s.maze != classic() || s.pacman.at != pacmanStart() || s.score != 1000 {
		t.Fatalf("after clearing level 1: level %d, paused %t, score %d", s.level, s.paused, s.score)
	}
	s.paused = false
	if d, due := s.NextTick(); d >= levelOf(1).interval || !due {
		t.Errorf("level 2 ticks every %v, want faster than level 1's %v", d, levelOf(1).interval)
	}

	s.level = Levels
	s = alone(emptied(s, pt(23, 12)))
	next, _ = s.Tick(game.StepRNG(1, 2))
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatPlayer}, Reason: ReasonCleared}
	if got, done := next.Outcome(); !done || !cmp.Equal(got, want) {
		t.Fatalf("clearing the last level: %+v, %t; want %+v", got, done, want)
	}
}

func TestTheGhostsOnTheLastDotAreMetBeforeTheLevelClears(t *testing.T) {
	lastDot := func(frightened bool) state {
		s := alone(running())
		s.pacman = pacman{at: pt(1, 2), facing: grid.Left}
		s = emptied(s, pt(1, 1))
		s.ghosts[0] = ghost{name: Blinky, at: pt(1, 1), facing: grid.Right, activity: roaming, frightened: frightened}
		return s
	}

	s, events := tickOnce(lastDot(false), 1)
	if diff := cmp.Diff([]game.Event{CaughtEvent{By: SeatPlayer, Ghost: Blinky, LivesLeft: 2}}, events); diff != "" || s.level != 1 {
		t.Fatalf("caught on the last dot: level %d; events (-want +got):\n%s", s.level, diff)
	}
	s.paused = false
	if s, events = tickOnce(s, 2); s.level != 2 {
		t.Fatalf("the round after: level %d, events %v; want the level cleared", s.level, events)
	}

	s, events = tickOnce(lastDot(true), 1)
	want := []game.Event{GhostEatenEvent{By: SeatPlayer, Ghost: Blinky, Points: GhostPoints}, LevelClearedEvent{By: SeatPlayer, Level: 1}}
	if diff := cmp.Diff(want, events); diff != "" || s.score != GhostPoints+DotPoints {
		t.Fatalf("a frightened ghost on the last dot: score %d; events (-want +got):\n%s", s.score, diff)
	}
}

func TestPauseTurnAndTheirLegality(t *testing.T) {
	s := newState()
	if _, due := s.NextTick(); due || !s.View(SeatPlayer).Paused || s.View(SeatPlayer).CanPause {
		t.Fatal("a new game is not waiting, paused, for its first turn")
	}
	next, _, err := s.Apply(SeatPlayer, turn(grid.Right), nil)
	if err != nil {
		t.Fatal(err)
	}
	v := next.View(SeatPlayer)
	if v.Paused || !v.CanPause || v.Pacman.NextTurn == nil || *v.Pacman.NextTurn != grid.Right {
		t.Fatalf("after a turn: %+v", v)
	}
	if d, due := next.(state).NextTick(); !due || d != levelOf(1).interval {
		t.Errorf("a running game ticks %v, %t", d, due)
	}
	paused, _, err := next.Apply(SeatPlayer, Action{Kind: ActionPause}, nil)
	if err != nil || !paused.View(SeatPlayer).Paused {
		t.Fatalf("pause: %v", err)
	}
	if core := next.(state).Pause(); !core.View(SeatPlayer).Paused {
		t.Error("the core's pause did not pause")
	}
	rejected := []struct {
		name   string
		state  game.State[Action, View]
		seat   game.Seat
		action Action
		want   string
	}{
		{"a pause while paused", paused, SeatPlayer, Action{Kind: ActionPause}, "already paused"},
		{"an unknown way", next, SeatPlayer, Action{Kind: ActionTurn, Direction: "north"}, `want up, down, left or right; got "north"`},
		{"an unknown action", next, SeatPlayer, Action{Kind: "jump"}, "unknown action: jump"},
		{"another seat", next, "ghost", turn(grid.Up), game.ErrNotYourTurn.Error()},
		{"a finished game", next.(state).Conclude(game.Result{Kind: game.ResultAborted}), SeatPlayer, turn(grid.Up), game.ErrNotYourTurn.Error()},
	}
	for _, tt := range rejected {
		_, _, err := tt.state.Apply(tt.seat, tt.action, nil)
		var illegal game.IllegalActionError
		if err == nil || !strings.Contains(err.Error(), tt.want) || !errors.As(err, &illegal) && !errors.Is(err, game.ErrNotYourTurn) {
			t.Errorf("%s: error %v, want %q", tt.name, err, tt.want)
		}
	}
}

func TestConcludeClosesEveryAction(t *testing.T) {
	s := running()
	done := s.Conclude(game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{}, Reason: game.ReasonResign})
	v := done.View(SeatPlayer)
	if v.CanTurn || v.CanPause || len(done.ToAct()) != 0 {
		t.Errorf("after an imposed result: %+v, to act %v", v, done.ToAct())
	}
	if _, due := done.(state).NextTick(); due {
		t.Error("a concluded game wants a tick")
	}
	won := s.over(game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatPlayer}, Reason: ReasonCleared})
	if kept, _ := won.Conclude(game.Result{Kind: game.ResultAborted}).Outcome(); kept.Reason != ReasonCleared {
		t.Errorf("Conclude replaced a result already reached with %s", kept.Reason)
	}
}

func TestViewShowsTheMazeOneBased(t *testing.T) {
	v := newState().View(SeatPlayer)
	if len(v.Maze) != Rows || len(v.Maze[0]) != Cols || v.Maze[3][1] != byte(TilePellet) {
		t.Fatalf("maze %d rows of %d, (4,2) %q", len(v.Maze), len(v.Maze[0]), v.Maze[3][1])
	}
	want := PacmanView{Actor: Actor{Row: 24, Col: 14, Facing: grid.Left}}
	if diff := cmp.Diff(want, v.Pacman); diff != "" {
		t.Errorf("pacman (-want +got):\n%s", diff)
	}
	modes := map[GhostName]GhostMode{Blinky: ModeScatter, Pinky: ModeHouse, Inky: ModeHouse, Clyde: ModeHouse}
	for _, g := range v.Ghosts {
		if g.Mode != modes[g.Name] {
			t.Errorf("%s starts %s, want %s", g.Name, g.Mode, modes[g.Name])
		}
	}
	if v.Lives != StartingLives || v.Level != 1 || v.Levels != Levels || v.FoodLeft != 244 || !v.CanTurn {
		t.Errorf("view %+v", v)
	}
}
