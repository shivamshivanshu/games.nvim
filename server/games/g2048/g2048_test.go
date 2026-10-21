package g2048

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const solver game.UserID = "alice"

func move(direction string) Action { return Action{Kind: ActionMove, Direction: direction} }

func newState(t *testing.T, seed game.Seed) state {
	t.Helper()
	st, seats, err := Rules{}.New(Opts{}, solver, nil, game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[game.UserID]game.Seat{solver: SeatSolver}, seats); diff != "" {
		t.Fatalf("seats (-want +got):\n%s", diff)
	}
	return st.(state)
}

func apply(t *testing.T, s state, action Action, seed game.Seed, seq game.Seq) (state, []game.Event) {
	t.Helper()
	next, events, err := s.Apply(SeatSolver, action, game.StepRNG(seed, seq))
	if err != nil {
		t.Fatalf("%+v: %v", action, err)
	}
	return next.(state), events
}

func tiles(b board) []int {
	var out []int
	for _, row := range b {
		for _, v := range row {
			if v != Empty {
				out = append(out, v)
			}
		}
	}
	return out
}

func TestNewSpawnsTwoTilesOfTwoOrFour(t *testing.T) {
	for seed := range game.Seed(50) {
		got := tiles(newState(t, seed).board)
		if len(got) != 2 || slices.ContainsFunc(got, func(v int) bool { return v != 2 && v != 4 }) {
			t.Errorf("seed %d: start tiles %v, want two of 2 or 4", seed, got)
		}
	}
	if newState(t, 1).board == newState(t, 2).board && newState(t, 2).board == newState(t, 3).board {
		t.Error("seeds 1, 2 and 3 start on the same board")
	}
}

func TestNewRefusesOpponents(t *testing.T) {
	_, _, err := Rules{}.New(Opts{}, solver, []game.UserID{"bob"}, game.StepRNG(1, 0))
	if !errors.As(err, new(game.BadRequestError)) {
		t.Errorf("got %v, want a bad request", err)
	}
}

func TestMergedPacksAndMergesEachTileOnceNearestTheEdgeFirst(t *testing.T) {
	cases := []struct {
		name       string
		line, want [side]int
		gained     int
	}{
		{"empty", [side]int{}, [side]int{}, 0},
		{"packs to the edge", [side]int{0, 0, 0, 2}, [side]int{2, 0, 0, 0}, 0},
		{"gap between equal tiles", [side]int{4, 0, 4, 0}, [side]int{8, 0, 0, 0}, 8},
		{"three equal: the pair nearest the edge merges", [side]int{2, 2, 2, 0}, [side]int{4, 2, 0, 0}, 4},
		{"four equal: two merges", [side]int{2, 2, 2, 2}, [side]int{4, 4, 0, 0}, 8},
		{"a merged tile does not merge again", [side]int{2, 2, 4, 0}, [side]int{4, 4, 0, 0}, 4},
		{"nor with the next merge", [side]int{4, 4, 8, 0}, [side]int{8, 8, 0, 0}, 8},
		{"unequal neighbours stay", [side]int{2, 4, 2, 4}, [side]int{2, 4, 2, 4}, 0},
		{"merge after a blocker", [side]int{8, 2, 0, 2}, [side]int{8, 4, 0, 0}, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, gained := merged(tc.line)
			if got != tc.want || gained != tc.gained {
				t.Errorf("merged(%v) = %v, %d; want %v, %d", tc.line, got, gained, tc.want, tc.gained)
			}
		})
	}
}

func TestSlidMergesTowardTheMove(t *testing.T) {
	start := board{
		{2, 2, 2, 0},
		{0, 0, 0, 0},
		{2, 0, 0, 0},
		{2, 0, 0, 0},
	}
	cases := []struct {
		direction Direction
		want      board
		gained    int
	}{
		{DirectionLeft, board{{4, 2, 0, 0}, {0, 0, 0, 0}, {2, 0, 0, 0}, {2, 0, 0, 0}}, 4},
		{DirectionRight, board{{0, 0, 2, 4}, {0, 0, 0, 0}, {0, 0, 0, 2}, {0, 0, 0, 2}}, 4},
		{DirectionUp, board{{4, 2, 2, 0}, {2, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}}, 4},
		{DirectionDown, board{{0, 0, 0, 0}, {0, 0, 0, 0}, {2, 0, 0, 0}, {4, 2, 2, 0}}, 4},
	}
	for _, tc := range cases {
		t.Run(string(tc.direction), func(t *testing.T) {
			got, gained := start.slid(tc.direction)
			if diff := cmp.Diff(tc.want, got); diff != "" || gained != tc.gained {
				t.Errorf("gained %d, want %d; board (-want +got):\n%s", gained, tc.gained, diff)
			}
		})
	}
}

func TestMovableListsOnlyDirectionsThatChangeTheBoard(t *testing.T) {
	cases := []struct {
		name  string
		board board
		want  []Direction
	}{
		{"one tile in the top-left corner", board{{2}}, []Direction{DirectionDown, DirectionRight}},
		{"a full row that can merge", board{{2, 2, 4, 8}}, []Direction{DirectionDown, DirectionLeft, DirectionRight}},
		{"a full board with no equal neighbours", stuck(), []Direction{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, tc.board.movable()); diff != "" {
				t.Errorf("movable (-want +got):\n%s", diff)
			}
		})
	}
}

func stuck() board {
	return board{
		{2, 4, 2, 4},
		{4, 2, 4, 2},
		{2, 4, 2, 4},
		{4, 2, 4, 2},
	}
}

func TestSpawnedFillsAnEmptyCellWithATwoNineTimesInTen(t *testing.T) {
	oneEmpty := stuck()
	oneEmpty[2][1] = Empty
	fours := 0
	const draws = 2000
	for seq := range game.Seq(draws) {
		got, tile := oneEmpty.spawned(game.StepRNG(7, seq))
		if tile.Row != 3 || tile.Col != 2 || got[2][1] != tile.Value {
			t.Fatalf("draw %d spawned %+v, want row 3 col 2 on the board", seq, tile)
		}
		switch tile.Value {
		case 4:
			fours++
		case 2:
		default:
			t.Fatalf("draw %d spawned %d", seq, tile.Value)
		}
	}
	if fours < draws/20 || fours > draws*3/20 {
		t.Errorf("%d fours in %d spawns, want about a tenth", fours, draws)
	}
}

func TestApplySlidesSpawnsAndScores(t *testing.T) {
	const seed game.Seed = 3
	s := state{board: board{{2, 2, 4, 4}}, score: 10}
	next, events := apply(t, s, move("left"), seed, 1)
	slid := board{{4, 8}}
	moved := events[0].(MovedEvent)
	if len(events) != 1 || moved.By != SeatSolver || moved.Direction != DirectionLeft || moved.Gained != 12 {
		t.Fatalf("events %+v, want one moved left gaining 12", events)
	}
	at := moved.Spawned
	if slid[at.Row-1][at.Col-1] != Empty {
		t.Errorf("spawned %+v on a cell the slide filled", at)
	}
	slid[at.Row-1][at.Col-1] = at.Value
	if diff := cmp.Diff(slid, next.board); diff != "" {
		t.Errorf("board (-want +got):\n%s", diff)
	}
	if next.score != 22 {
		t.Errorf("score %d, want 22", next.score)
	}
	again, _ := apply(t, s, move("left"), seed, 1)
	if again.board != next.board {
		t.Error("the same step's rng spawned differently")
	}
}

func TestRejectedActions(t *testing.T) {
	finished := state{board: stuck(), result: noMovesResult(false), finished: true}
	cases := []struct {
		name    string
		state   state
		seat    game.Seat
		action  Action
		wantErr error
	}{
		{"a move that changes nothing", state{board: board{{2}}}, SeatSolver, move("up"), game.IllegalActionError{Msg: "no tile can move up"}},
		{"unknown direction", state{board: board{{2}}}, SeatSolver, move("north"), game.IllegalActionError{Msg: `want up, down, left or right; got "north"`}},
		{"direction is case sensitive like the catalogue", state{board: board{{2}}}, SeatSolver, move("Down"), game.IllegalActionError{Msg: `want up, down, left or right; got "Down"`}},
		{"unknown kind", state{board: board{{2}}}, SeatSolver, Action{Kind: "slide", Direction: "down"}, game.IllegalActionError{Msg: "unknown action: slide"}},
		{"another seat", state{board: board{{2}}}, "other", move("down"), game.ErrNotYourTurn},
		{"after the game ends", finished, SeatSolver, move("down"), game.ErrNotYourTurn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tc.state.Apply(tc.seat, tc.action, game.StepRNG(1, 1))
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// lastMove leaves the board full but for (0,3) whatever spawns there: no equal neighbours remain.
func lastMove(row0 [side]int) board {
	return board{
		row0,
		{2, 4, 32, 64},
		{4, 2, 64, 32},
		{2, 4, 32, 64},
	}
}

func TestReachingTheGoalIsMarkedOnceAndPlayGoesOn(t *testing.T) {
	s := state{board: board{{1024, 1024}, {2048}}}
	next, events := apply(t, s, move("left"), 1, 1)
	if !next.reached || next.finished {
		t.Fatalf("reached %t finished %t, want reached and playing on", next.reached, next.finished)
	}
	if len(events) != 2 || events[1] != (ReachedEvent{By: SeatSolver, Tile: Goal}) {
		t.Errorf("events %+v, want moved then reached %d", events, Goal)
	}
	if !next.View(SeatSolver).Reached {
		t.Error("view does not mark the goal reached")
	}
	again, events := apply(t, next, move("up"), 1, 2)
	if len(events) != 1 || !again.reached {
		t.Errorf("a move after the goal: events %+v, reached %t; want one moved event and still reached", events, again.reached)
	}
	before, _ := apply(t, state{board: board{{512, 512}}}, move("left"), 1, 1)
	if before.reached || before.View(SeatSolver).Reached {
		t.Error("a 1024 marks the goal reached")
	}
}

func TestNoMovesEndsTheGame(t *testing.T) {
	cases := []struct {
		name    string
		state   state
		want    game.Result
		reached bool
	}{
		{
			name:  "without the goal it is a loss",
			state: state{board: lastMove([side]int{32, 64, 8, 8})},
			want:  game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{}, Reason: ReasonNoMoves},
		},
		{
			name:    "the move that reaches the goal can also end it, a win",
			state:   state{board: lastMove([side]int{32, 64, 1024, 1024})},
			want:    game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonNoMoves},
			reached: true,
		},
		{
			name:    "a goal reached earlier wins",
			state:   state{board: lastMove([side]int{32, 2048, 8, 8}), reached: true},
			want:    game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonNoMoves},
			reached: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for seq := range game.Seq(20) {
				next, _ := apply(t, tc.state, move("left"), 1, seq)
				got, finished := next.Outcome()
				if diff := cmp.Diff(tc.want, got); !finished || diff != "" {
					t.Fatalf("seq %d: finished %t, result (-want +got):\n%s", seq, finished, diff)
				}
				view := next.View(SeatSolver)
				if len(view.Movable) != 0 || view.Reached != tc.reached || len(next.ToAct()) != 0 {
					t.Errorf("seq %d: view %+v, to act %v; want no moves, reached %t", seq, view, next.ToAct(), tc.reached)
				}
			}
		})
	}
}

func TestConcludeStopsOfferingMovesAndKeepsAReachedResult(t *testing.T) {
	playing := state{board: board{{2}}, reached: true}
	resigned := game.ForfeitToRemaining([]game.Seat{SeatSolver}, SeatSolver, game.ReasonResign)
	concluded := playing.Conclude(resigned)
	if got, finished := concluded.Outcome(); !finished || !cmp.Equal(got, resigned) {
		t.Errorf("concluded outcome %+v %t, want %+v", got, finished, resigned)
	}
	if view := concluded.View(SeatSolver); len(view.Movable) != 0 || !view.Reached {
		t.Errorf("concluded view %+v, want no moves and the goal still marked", view)
	}
	ended := state{board: stuck(), result: noMovesResult(true), finished: true}
	if got, _ := ended.Conclude(resigned).Outcome(); !cmp.Equal(got, noMovesResult(true)) {
		t.Errorf("Conclude replaced a reached result with %+v", got)
	}
}

func TestViewWire(t *testing.T) {
	raw, err := json.Marshal(state{board: board{{2, 0, 0, 4}}, score: 8}.View(SeatSolver))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"cells":[[2,0,0,4],[0,0,0,0],[0,0,0,0],[0,0,0,0]],"score":8,"reached":false,"movable":["down","left","right"]}`
	if string(raw) != want {
		t.Errorf("view\n got %s\nwant %s", raw, want)
	}
	over, err := json.Marshal(state{board: stuck(), finished: true}.View(SeatSolver))
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err := json.Unmarshal(over, &view); err != nil {
		t.Fatal(err)
	}
	if movable, ok := view["movable"].([]any); !ok || len(movable) != 0 {
		t.Errorf("finished movable = %v, want []", view["movable"])
	}
}
