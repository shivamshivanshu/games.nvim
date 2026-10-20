package checkers

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
)

func move(text string) Action { return Action{Kind: ActionMove, Text: text} }

func newGame(t *testing.T, color ColorChoice, seed game.Seed) (game.ErasedState, map[game.UserID]game.Seat) {
	t.Helper()
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(json.RawMessage(`{"color":"` + string(color) + `"}`))
	if err != nil {
		t.Fatal(err)
	}
	st, seats, err := g.New(opts, alice, []game.UserID{bob}, game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	return st, seats
}

func apply(t *testing.T, st game.State[Action, View], seat game.Seat, text string) (game.State[Action, View], []game.Event) {
	t.Helper()
	next, events, err := st.Apply(seat, move(text), nil)
	if err != nil {
		t.Fatalf("%s: %v", text, err)
	}
	return next, events
}

func TestColorOption(t *testing.T) {
	cases := []struct {
		color ColorChoice
		seed  game.Seed
		want  game.Seat
	}{
		{ColorBlack, 1, SeatBlack},
		{ColorBlack, 2, SeatBlack},
		{ColorWhite, 1, SeatWhite},
		{ColorWhite, 2, SeatWhite},
		{ColorRandom, 1, SeatWhite},
		{ColorRandom, 6, SeatBlack},
	}
	for _, c := range cases {
		st, seats := newGame(t, c.color, c.seed)
		if seats[alice] != c.want || seats[bob] != opponent(c.want) {
			t.Errorf("%s seed %d: got %v", c.color, c.seed, seats)
		}
		if got := st.ToAct(); !slices.Equal(got, []game.Seat{SeatBlack}) {
			t.Errorf("%s seed %d: black moves first, got %v", c.color, c.seed, got)
		}
	}
}

func TestApplyLeavesEarlierStateUnchanged(t *testing.T) {
	before := newState(StartBoard(), SeatBlack, 0)
	earlier := []string{"11-15", "23-19", "8-11"}
	before.history = append(make([]string, 0, len(earlier)+1), earlier...)
	want := before.View(SeatBlack)
	first, _ := apply(t, before, SeatBlack, "9-13")
	second, _ := apply(t, before, SeatBlack, "12-16")
	for text, after := range map[string]game.State[Action, View]{"9-13": first, "12-16": second} {
		if got := after.View(SeatBlack).History; !slices.Equal(got, append(slices.Clone(earlier), text)) {
			t.Errorf("after %s: history %v", text, got)
		}
	}
	if diff := cmp.Diff(want, before.View(SeatBlack)); diff != "" {
		t.Errorf("Apply changed its receiver (-before +after):\n%s", diff)
	}
}

func TestRejectedActions(t *testing.T) {
	capture := newState(boardOf(t, "b:9,12 w:14,23 W:32"), SeatBlack, 0)
	cases := []struct {
		name    string
		state   state
		seat    game.Seat
		action  Action
		wantMsg string
	}{
		{"unknown kind", newState(StartBoard(), SeatBlack, 0), SeatBlack, Action{Kind: "jump"}, "unknown action: jump"},
		{"garbage", newState(StartBoard(), SeatBlack, 0), SeatBlack, move("e4"), "illegal move: e4"},
		{"backward man", newState(boardOf(t, "b:14 w:30"), SeatBlack, 0), SeatBlack, move("14-9"), "illegal move: 14-9"},
		{"capture written as a step", capture, SeatBlack, move("9-18"), "illegal move 9-18: a capture is mandatory: 9x18x27"},
		{"step while a capture exists", capture, SeatBlack, move("12-16"), "illegal move 12-16: a capture is mandatory: 9x18x27"},
		{"incomplete jump", capture, SeatBlack, move("9x18"), "illegal move 9x18: the jump must continue, as in 9x18x27"},
		{"jump past the end", capture, SeatBlack, move("9x18x27x31"), "illegal move 9x18x27x31: a capture is mandatory: 9x18x27"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := c.state.Apply(c.seat, c.action, nil)
			var illegalErr game.IllegalActionError
			if !errors.As(err, &illegalErr) || illegalErr.Msg != c.wantMsg {
				t.Errorf("got %v, want %q", err, c.wantMsg)
			}
		})
	}
	if _, _, err := capture.Apply(SeatWhite, move("32-27"), nil); !errors.Is(err, game.ErrNotYourTurn) {
		t.Errorf("white out of turn: got %v", err)
	}
}

func TestAcceptsSurroundingSpace(t *testing.T) {
	next, _ := apply(t, newState(StartBoard(), SeatBlack, 0), SeatBlack, " 11-15 ")
	if got := next.View(SeatWhite).History; !slices.Equal(got, []string{"11-15"}) {
		t.Errorf("history %v", got)
	}
}

func TestMoveEventAndView(t *testing.T) {
	st, events := apply(t, newState(boardOf(t, "b:22 w:26,27 W:4"), SeatBlack, 0), SeatBlack, "22x31")
	if diff := cmp.Diff([]game.Event{MovedEvent{By: SeatBlack, Move: "22x31", Crowned: true}}, events); diff != "" {
		t.Errorf("events (-want +got):\n%s", diff)
	}
	raw, err := json.Marshal(st.View(SeatBlack))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"board":["","","","W","","","","","","","","","","","","","","","","","","","","","","","w","","","","B",""],` +
		`"legal":[[4,8],[27,23],[27,24]],"last":[22,31],"history":["22x31"]}`
	if string(raw) != want {
		t.Errorf("view\n got %s\nwant %s", raw, want)
	}
	if got := st.ToAct(); !slices.Equal(got, []game.Seat{SeatWhite}) {
		t.Errorf("crowning must end the turn; to act %v", got)
	}
}

func TestStartViewHasNoNullArrays(t *testing.T) {
	raw, err := json.Marshal(newState(StartBoard(), SeatBlack, 0).View(SeatBlack))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "null") || !strings.Contains(string(raw), `"last":[],"history":[]`) {
		t.Errorf("view %s", raw)
	}
}

func TestEndings(t *testing.T) {
	cases := []struct {
		name       string
		layout     string
		quietPlies int
		move       string
		want       *game.Result
	}{
		{"capturing the last piece wins", "b:9 w:14", 0, "9x18",
			&game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatBlack}, Reason: ReasonNoPieces}},
		{"blocking the last man wins", "b:1,2,5,14 w:9", 0, "1-6",
			&game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatBlack}, Reason: ReasonNoMoves}},
		{"80th quiet ply draws", "B:1 W:32", drawQuietPlies - 1, "1-5",
			&game.Result{Kind: game.ResultDraw, Reason: ReasonFortyMove}},
		{"79th quiet ply plays on", "B:1 W:32", drawQuietPlies - 2, "1-5", nil},
		{"a man move resets the count", "b:1 B:2 W:32", drawQuietPlies - 1, "1-5", nil},
		{"a crowning is a man move", "b:25 W:1", drawQuietPlies - 1, "25-29", nil},
		{"a capture resets the count", "B:1 w:6 W:32", drawQuietPlies - 1, "1x10", nil},
		{"a win on the 80th ply beats the draw", "B:1 b:2,5,14 w:9", drawQuietPlies - 1, "1-6",
			&game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatBlack}, Reason: ReasonNoMoves}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, _ := apply(t, newState(boardOf(t, c.layout), SeatBlack, c.quietPlies), SeatBlack, c.move)
			got, finished := st.Outcome()
			if c.want == nil {
				if finished {
					t.Fatalf("want play on, got %+v", got)
				}
				return
			}
			if diff := cmp.Diff(*c.want, got); !finished || diff != "" {
				t.Errorf("finished=%t (-want +got):\n%s", finished, diff)
			}
			if len(st.ToAct()) != 0 || len(st.View(SeatWhite).Legal) != 0 {
				t.Errorf("finished game still offers moves: to act %v", st.ToAct())
			}
		})
	}
}

type pick func(legal []Move) Move

func firstLegal(legal []Move) Move { return legal[0] }

func lastLegal(legal []Move) Move { return legal[len(legal)-1] }

// selfPlay plays a whole game from the start, choosing each move with choose.
func selfPlay(t *testing.T, choose pick) []gametest.Step[Action] {
	t.Helper()
	st := newState(StartBoard(), SeatBlack, 0)
	var steps []gametest.Step[Action]
	for !st.finished {
		step := gametest.Step[Action]{Seat: st.toMove, Action: move(choose(st.legal).Text())}
		next, _, err := st.Apply(step.Seat, step.Action, nil)
		if err != nil {
			t.Fatal(err)
		}
		st, steps = next.(state), append(steps, step)
	}
	return steps
}

func TestConformance(t *testing.T) {
	script := func(name string, choose pick, want game.Result) gametest.Script[Opts, Action] {
		steps := selfPlay(t, choose)
		misplay := gametest.Step[Action]{Seat: SeatWhite, Action: move("9-13"), WantErr: true}
		return gametest.Script[Opts, Action]{
			Name:       name,
			Opts:       Opts{TimeControl: game.TimeControlNone, Color: ColorBlack},
			Challenger: alice,
			Invitees:   []game.UserID{bob},
			Seed:       1,
			Steps:      append([]gametest.Step[Action]{steps[0], misplay}, steps[1:]...),
			WantResult: &want,
		}
	}
	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		script("first legal move each turn", firstLegal, game.Result{Kind: game.ResultDraw, Reason: ReasonFortyMove}),
		script("last legal move each turn", lastLegal, game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatWhite}, Reason: ReasonNoPieces}),
	})
}
