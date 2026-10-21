package checkers

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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
		if seats[alice] != c.want || seats[bob] != sides.Other(c.want) {
			t.Errorf("%s seed %d: got %v", c.color, c.seed, seats)
		}
		if got := st.ToAct(); !slices.Equal(got, []game.Seat{SeatBlack}) {
			t.Errorf("%s seed %d: black moves first, got %v", c.color, c.seed, got)
		}
	}
}

func TestColorDefaultsToRandom(t *testing.T) {
	opts, err := game.Adapt(Rules{}).ResolveOptions(json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := opts[game.OptionColor]; got != game.OptionValue(ColorRandom) {
		t.Errorf("default color %q", got)
	}
}

func TestApplyLeavesEarlierStateUnchanged(t *testing.T) {
	before := newState(StartBoard(), SeatBlack, 0)
	earlier := []string{"11-15", "23-19", "8-11"}
	before.history = append(make([]string, 0, len(earlier)+1), earlier...)
	want := before.View(SeatBlack)
	first, _ := gametest.MustApply(t, Rules{}.Info(), before, SeatBlack, move("9-13"), nil)
	second, _ := gametest.MustApply(t, Rules{}.Info(), before, SeatBlack, move("12-16"), nil)
	for text, after := range map[string]game.State[Action, View]{"9-13": first, "12-16": second} {
		if got := after.View(SeatBlack).History; !slices.Equal(got, append(slices.Clone(earlier), text)) {
			t.Errorf("after %s: history %v", text, got)
		}
	}
	if diff := cmp.Diff(want, before.View(SeatBlack)); diff != "" {
		t.Errorf("Apply changed its receiver (-before +after):\n%s", diff)
	}
}

func notationError(text string) string {
	return fmt.Sprintf("illegal move %q: write %s", text, notationHint)
}

func TestRejectedActions(t *testing.T) {
	start := newState(StartBoard(), SeatBlack, 0)
	capture := newState(boardOf(t, "b:9,12 w:14,23 W:32"), SeatBlack, 0)
	cases := []struct {
		name    string
		state   state
		seat    game.Seat
		action  Action
		wantMsg string
	}{
		{"unknown kind", newState(StartBoard(), SeatBlack, 0), SeatBlack, Action{Kind: "jump"}, "unknown action: jump"},
		{"garbage", start, SeatBlack, move("e4"), notationError("e4")},
		{"a lone square", start, SeatBlack, move("9"), notationError("9")},
		{"a lone square while a capture exists", capture, SeatBlack, move("9"), notationError("9")},
		{"square past 32", start, SeatBlack, move("99-98"), notationError("99-98")},
		{"leading zero", start, SeatBlack, move("01-05"), notationError("01-05")},
		{"three digits", start, SeatBlack, move("111-15"), notationError("111-15")},
		{"wrong capture", capture, SeatBlack, move("9x1"), "illegal move 9x1: the captures are 9x18x27"},
		{"capture with none to make", start, SeatBlack, move("9x18"), "illegal move: 9x18"},
		{"backward man", newState(boardOf(t, "b:14 w:30"), SeatBlack, 0), SeatBlack, move("14-9"), "illegal move: 14-9"},
		{"capture written as a step", capture, SeatBlack, move("9-18"), "illegal move 9-18: a capture is mandatory: 9x18x27"},
		{"step while a capture exists", capture, SeatBlack, move("12-16"), "illegal move 12-16: a capture is mandatory: 9x18x27"},
		{"incomplete jump", capture, SeatBlack, move("9x18"), "illegal move 9x18: the jump must continue, as in 9x18x27"},
		{"jump past the end", capture, SeatBlack, move("9x18x27x31"), "illegal move 9x18x27x31: the captures are 9x18x27"},
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
}

func TestAcceptsSurroundingSpace(t *testing.T) {
	next, _ := gametest.MustApply(t, Rules{}.Info(), newState(StartBoard(), SeatBlack, 0), SeatBlack, move(" 11-15 "), nil)
	if got := next.View(SeatWhite).History; !slices.Equal(got, []string{"11-15"}) {
		t.Errorf("history %v", got)
	}
}

func TestMoveEventAndView(t *testing.T) {
	st, events := gametest.MustApply(t, Rules{}.Info(), newState(boardOf(t, "b:22 w:26,27 W:4"), SeatBlack, 0), SeatBlack, move("22x31"), nil)
	if diff := cmp.Diff([]game.Event{MovedEvent{By: SeatBlack, Move: "22x31", Crowned: true}}, events); diff != "" {
		t.Errorf("events (-want +got):\n%s", diff)
	}
	board := `"board":["","","","W","","","","","","","","","","","","","","","","","","","","","","","w","","","","B",""]`
	views := map[game.Seat]string{
		SeatWhite: `{` + board + `,"legal":[[4,8],[27,23],[27,24]],"last":[22,31],"history":["22x31"]}`,
		SeatBlack: `{` + board + `,"legal":[],"last":[22,31],"history":["22x31"]}`,
	}
	for seat, want := range views {
		raw, err := json.Marshal(st.View(seat))
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != want {
			t.Errorf("%s view\n got %s\nwant %s", seat, raw, want)
		}
	}
	if got := st.ToAct(); !slices.Equal(got, []game.Seat{SeatWhite}) {
		t.Errorf("crowning must end the turn; to act %v", got)
	}
}

func TestConcludeStopsOfferingMoves(t *testing.T) {
	agreed := game.Result{Kind: game.ResultDraw, Reason: game.ReasonDrawAgreed}
	concluded := newState(StartBoard(), SeatBlack, 0).Conclude(agreed)
	if got, finished := concluded.Outcome(); !finished || !cmp.Equal(got, agreed) {
		t.Errorf("outcome %+v, finished=%t", got, finished)
	}
	if len(concluded.ToAct()) != 0 || len(concluded.View(SeatBlack).Legal) != 0 {
		t.Errorf("concluded game still offers moves to %v", concluded.ToAct())
	}
	won := newState(boardOf(t, "b:9"), SeatWhite, 0)
	if got, _ := won.Conclude(agreed).Outcome(); got.Reason != ReasonNoPieces {
		t.Errorf("Conclude overrode a finished result: %+v", got)
	}
}

func TestMovePatternMatchesNotation(t *testing.T) {
	pattern := regexp.MustCompile(movePattern)
	for _, steps := range [][]gametest.Step[Action]{selfPlay(t, firstLegal), selfPlay(t, lastLegal)} {
		for _, step := range steps {
			if !pattern.MatchString(step.Action.Text) {
				t.Errorf("pattern rejects legal move %q", step.Action.Text)
			}
		}
	}
	for _, text := range []string{"", "e4", "11", "11-15-19", "11x", "11-15x19", "99-98", "01-05", "111-15", "0-4", "33x24"} {
		if pattern.MatchString(text) {
			t.Errorf("pattern accepts %q", text)
		}
	}
}

func TestPatternAndParserAgreeOnNotation(t *testing.T) {
	pattern := regexp.MustCompile(movePattern)
	legal := StartBoard().LegalMoves(SeatBlack)
	for _, text := range []string{"11-15", " 11-15 ", "\t11-15\n", "\v11-15", "11-15\u00a0", "32-28", "01-05", "9", "9x1"} {
		_, err := matchMove(text, legal)
		parsed := err == nil || !strings.Contains(err.Error(), notationHint)
		if parsed != pattern.MatchString(text) {
			t.Errorf("%q: parser reads notation %t, pattern matches %t", text, parsed, pattern.MatchString(text))
		}
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
			st, _ := gametest.MustApply(t, Rules{}.Info(), newState(boardOf(t, c.layout), SeatBlack, c.quietPlies), SeatBlack, move(c.move), nil)
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
	for !st.Over() {
		step := gametest.Step[Action]{Seat: st.toMove, Action: move(choose(st.legal).Text())}
		st, _ = gametest.MustApply(t, Rules{}.Info(), st, step.Seat, step.Action, nil)
		steps = append(steps, step)
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
