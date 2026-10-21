package fifteen

import (
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

const solo game.UserID = "alice"

func sizes() []Size { return []Size{Size3, Size4} }

// fixture is a 3x3 state with tiles laid out row-major, Blank the empty square.
func fixture(tiles ...int) state {
	return state{board: board{side: 3, tiles: tiles}}
}

// nearlySolved has the blank at row 3, column 2: 8 sliding left solves it.
func nearlySolved() state { return fixture(1, 2, 3, 4, 5, 6, 7, Blank, 8) }

func slide(tile int) Action { return Action{Kind: ActionSlide, Tile: strconv.Itoa(tile)} }
func move(direction grid.Direction) Action {
	return Action{Kind: ActionMove, Direction: string(direction)}
}

func mustApply(t *testing.T, s state, actions ...Action) (state, []game.Event) {
	t.Helper()
	var events []game.Event
	for _, action := range actions {
		next, emitted, err := s.Apply(SeatSolver, action, nil)
		if err != nil {
			t.Fatalf("Apply(%+v): %v", action, err)
		}
		s, events = next.(state), emitted
	}
	return s, events
}

func TestShuffleIsSeededSolvableAndNeverSolved(t *testing.T) {
	for _, size := range sizes() {
		side, _ := size.side()
		distinct := map[string]bool{}
		const games = 200
		for seed := range game.Seed(games) {
			b := shuffled(side, game.StepRNG(seed, 0).Perm)
			if sorted := slices.Sorted(slices.Values(b.tiles)); !slices.Equal(sorted, slices.Sorted(slices.Values(solvedBoard(side).tiles))) {
				t.Fatalf("%s seed %d: tiles %v are not each tile once", size, seed, b.tiles)
			}
			if !b.solvable() || b.solved() {
				t.Errorf("%s seed %d: %v solvable %t, solved %t", size, seed, b.tiles, b.solvable(), b.solved())
			}
			if again := shuffled(side, game.StepRNG(seed, 0).Perm); !slices.Equal(again.tiles, b.tiles) {
				t.Errorf("%s seed %d: shuffled twice to %v and %v", size, seed, b.tiles, again.tiles)
			}
			distinct[string(tileBytes(b.tiles))] = true
		}
		if len(distinct) < games-5 {
			t.Errorf("%s: %d distinct shuffles from %d seeds", size, len(distinct), games)
		}
	}
}

func TestShuffleDrawsAgainWhenItWouldStartSolved(t *testing.T) {
	scrambled := []int{4, 1, 3, 7, 2, 6, 0, 5, 8}
	tests := []struct {
		name  string
		draws [][]int
	}{
		{"a solved draw", [][]int{{1, 2, 3, 4, 5, 6, 7, 8, Blank}, scrambled}},
		{"an unsolvable draw its swap would solve", [][]int{{2, 1, 3, 4, 5, 6, 7, 8, Blank}, scrambled}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drawn := 0
			perm := func(int) []int {
				drawn++
				return slices.Clone(tt.draws[drawn-1])
			}
			if got := shuffled(3, perm); !slices.Equal(got.tiles, scrambled) || drawn != 2 {
				t.Errorf("shuffled to %v after %d draws; want %v after 2", got.tiles, drawn, scrambled)
			}
		})
	}
}

// TestSolvableIsExactlyReachableOn3x3 walks every arrangement reachable from the solved one:
// each must be solvable, and swapping two tiles, which reaches the other half, must not be.
func TestSolvableIsExactlyReachableOn3x3(t *testing.T) {
	start := solvedBoard(3)
	seen := map[string]bool{string(tileBytes(start.tiles)): true}
	pending := []board{start}
	for len(pending) > 0 {
		b := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if !b.solvable() || b.swappedTiles().solvable() {
			t.Fatalf("%v is reachable: solvable %t, swapped solvable %t", b.tiles, b.solvable(), b.swappedTiles().solvable())
		}
		for _, m := range b.movable() {
			next := b.slid(m.Tile)
			if key := string(tileBytes(next.tiles)); !seen[key] {
				seen[key] = true
				pending = append(pending, next)
			}
		}
	}
	const half = 362880 / 2
	if len(seen) != half {
		t.Errorf("reached %d arrangements, want %d", len(seen), half)
	}
}

func TestSolvableOn4x4(t *testing.T) {
	loyd := board{side: 4, tiles: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 15, 14, Blank}}
	if loyd.solvable() {
		t.Error("Loyd's 14-15 puzzle is solvable")
	}
	walked := solvedBoard(4)
	rng := game.StepRNG(3, 0)
	for range 301 {
		movable := walked.movable()
		walked = walked.slid(movable[rng.IntN(len(movable))].Tile)
	}
	if !walked.solvable() || walked.swappedTiles().solvable() {
		t.Errorf("%v after a walk: solvable %t, swapped solvable %t", walked.tiles, walked.solvable(), walked.swappedTiles().solvable())
	}
}

func TestSlideAndMoveShiftATileIntoTheBlank(t *testing.T) {
	tests := []struct {
		name      string
		action    Action
		wantTiles [][]int
		wantEvent SlidEvent
	}{
		{"slide by number", slide(5), [][]int{{1, 2, 3}, {4, Blank, 6}, {7, 5, 8}}, SlidEvent{By: SeatSolver, Tile: 5, Direction: grid.Down}},
		{"slide with spaces", Action{Kind: ActionSlide, Tile: " 7 "}, [][]int{{1, 2, 3}, {4, 5, 6}, {Blank, 7, 8}}, SlidEvent{By: SeatSolver, Tile: 7, Direction: grid.Right}},
		{"move down", move(grid.Down), [][]int{{1, 2, 3}, {4, Blank, 6}, {7, 5, 8}}, SlidEvent{By: SeatSolver, Tile: 5, Direction: grid.Down}},
		{"move right", move(grid.Right), [][]int{{1, 2, 3}, {4, 5, 6}, {Blank, 7, 8}}, SlidEvent{By: SeatSolver, Tile: 7, Direction: grid.Right}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, events := mustApply(t, nearlySolved(), tt.action)
			view := next.View(SeatSolver)
			if diff := cmp.Diff(tt.wantTiles, view.Tiles); diff != "" {
				t.Errorf("tiles (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff([]game.Event{tt.wantEvent}, events); diff != "" {
				t.Errorf("events (-want +got):\n%s", diff)
			}
			if _, finished := next.Outcome(); view.Moves != 1 || finished {
				t.Errorf("moves %d, finished %t; want 1, playing", view.Moves, finished)
			}
		})
	}
}

func TestViewListsTheMovableTilesUntilTheEnd(t *testing.T) {
	want := []Movable{{Tile: 5, Direction: grid.Down}, {Tile: 7, Direction: grid.Right}, {Tile: 8, Direction: grid.Left}}
	if diff := cmp.Diff(want, nearlySolved().View(SeatSolver).Movable); diff != "" {
		t.Errorf("movable (-want +got):\n%s", diff)
	}
	corner := fixture(Blank, 1, 2, 3, 4, 5, 6, 7, 8)
	want = []Movable{{Tile: 1, Direction: grid.Left}, {Tile: 3, Direction: grid.Up}}
	if diff := cmp.Diff(want, corner.View(SeatSolver).Movable); diff != "" {
		t.Errorf("corner movable (-want +got):\n%s", diff)
	}
	over := nearlySolved().Conclude(game.ForfeitToRemaining([]game.Seat{SeatSolver}, SeatSolver, game.ReasonResign))
	if got := over.View(SeatSolver).Movable; got == nil || len(got) != 0 || len(over.ToAct()) != 0 {
		t.Errorf("after giving up: movable %v, to act %v; want [] and none", got, over.ToAct())
	}
}

func TestSolvingWinsAndCountsTheMoves(t *testing.T) {
	s, _ := mustApply(t, nearlySolved(), slide(5), slide(5), move(grid.Left))
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonSolved}
	if got, finished := s.Outcome(); !finished || !cmp.Equal(got, want) {
		t.Errorf("outcome = %+v, %t; want %+v", got, finished, want)
	}
	view := s.View(SeatSolver)
	if view.Moves != 3 || len(view.Movable) != 0 || len(s.ToAct()) != 0 {
		t.Errorf("moves %d, movable %v, to act %v; want 3 and none", view.Moves, view.Movable, s.ToAct())
	}
	if again := s.Conclude(game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}); !cmp.Equal(again.(state).result, want) {
		t.Errorf("Conclude replaced the reached result with %+v", again.(state).result)
	}
}

func TestIllegalActionsAreRefusedWithAReason(t *testing.T) {
	tests := []struct {
		action Action
		want   string
	}{
		{slide(3), "tile 3 is not next to the blank"},
		{slide(9), "no tile 9 on a 3x3 board"},
		{Action{Kind: ActionSlide, Tile: "0"}, `want a tile number, e.g. "7"; got "0"`},
		{Action{Kind: ActionSlide, Tile: "16"}, `want a tile number, e.g. "7"; got "16"`},
		{Action{Kind: ActionSlide}, `want a tile number, e.g. "7"; got ""`},
		{move(grid.Up), "no tile can move up"},
		{Action{Kind: ActionMove, Direction: "Up"}, `want up, down, left or right; got "Up"`},
		{Action{Kind: "jump"}, "unknown action: jump"},
	}
	for _, tt := range tests {
		_, _, err := nearlySolved().Apply(SeatSolver, tt.action, nil)
		if !errors.As(err, new(game.IllegalActionError)) || err.Error() != tt.want {
			t.Errorf("%+v: got %v, want illegal action %q", tt.action, err, tt.want)
		}
	}
}

func TestNewSeatsOneAndRefusesUnknownSizes(t *testing.T) {
	for _, size := range sizes() {
		st, seats, err := Rules{}.New(Opts{Size: size}, solo, nil, game.StepRNG(1, 0))
		side, _ := size.side()
		if err != nil || len(st.View(SeatSolver).Tiles) != side || !cmp.Equal(seats, map[game.UserID]game.Seat{solo: SeatSolver}) {
			t.Errorf("%s: err %v, seats %v", size, err, seats)
		}
	}
	if _, _, err := (Rules{}).New(Opts{Size: "5x5"}, solo, nil, game.StepRNG(1, 0)); !errors.As(err, new(game.BadRequestError)) {
		t.Errorf("unknown size: got %v, want a bad request", err)
	}
}
