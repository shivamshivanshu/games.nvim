package reversi

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
	"github.com/shivamshivanshu/games.nvim/server/games/discs"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
)

// wipeout is the shortest game: white has no disc left after black's ninth.
func wipeout() []string { return []string{"d3", "c3", "b3", "d2", "e1", "d6", "d7", "e3", "f4"} }

func place(cell string) discs.Action { return discs.Action{Kind: discs.ActionPlace, Cell: cell} }

func coords() grid.Coords { return discs.NewBoard(boardSize).Coords() }

func newGame(t *testing.T) game.State[discs.Action, View] {
	t.Helper()
	return gametest.NewState[game.State[discs.Action, View]](t, Rules{}, game.TurnOpts{First: game.FirstMoverMe}, []game.UserID{alice, bob}, 1)
}

// play places each cell for whichever seat is to move, so passes need no step.
func play(t *testing.T, cells ...string) game.State[discs.Action, View] {
	t.Helper()
	return playFrom(t, newGame(t), cells...)
}

func playFrom(t *testing.T, st game.State[discs.Action, View], cells ...string) game.State[discs.Action, View] {
	t.Helper()
	for _, cell := range cells {
		st, _ = gametest.MustApply(t, Rules{}.Info(), st, st.ToAct()[0], place(cell), nil)
	}
	return st
}

// steps places each cell for whichever seat is to move after the cells before it.
func steps(t *testing.T, cells ...string) []gametest.Step[discs.Action] {
	t.Helper()
	st := newGame(t)
	out := make([]gametest.Step[discs.Action], len(cells))
	for i, cell := range cells {
		out[i] = gametest.Step[discs.Action]{Seat: st.ToAct()[0], Action: place(cell)}
		st = playFrom(t, st, cell)
	}
	return out
}

// firstLegalGame plays the first legal cell, row by row, for whichever seat is to move until
// the game ends: 60 discs, black blocked and passed four times, white ahead 45 to 19.
func firstLegalGame(t *testing.T) []string {
	t.Helper()
	st := newGame(t)
	var cells []string
	for len(st.ToAct()) > 0 {
		seat := st.ToAct()[0]
		cell := coords().Name(legalPoints(st.(state).board, seat)[0])
		cells = append(cells, cell)
		st = playFrom(t, st, cell)
	}
	return cells
}

// position is a state with rows drawn as B, W and ., and seat to move.
func position(seat game.Seat, rows ...string) state {
	b := discs.NewBoard(boardSize)
	for r, row := range rows {
		for c, ch := range row {
			switch ch {
			case 'B':
				b = b.With(discs.SeatBlack, grid.Point{Row: r, Col: c})
			case 'W':
				b = b.With(discs.SeatWhite, grid.Point{Row: r, Col: c})
			}
		}
	}
	return state{Turn: game.TurnTo(seat), board: b}
}

func cells(names ...string) []grid.Cell {
	out := []grid.Cell{}
	for _, name := range names {
		p, ok := coords().Parse(name)
		if !ok {
			panic("bad cell " + name)
		}
		out = append(out, p.Cell())
	}
	return out
}

func TestNewSeatsTheFirstMoverAsBlack(t *testing.T) {
	gametest.SeatsFirstMover(t, Rules{}, func(first game.FirstMover) game.TurnOpts { return game.TurnOpts{First: first} }, discs.SeatBlack)
}

func TestStartPosition(t *testing.T) {
	st := newGame(t)
	want := View{
		Cells: [][]game.Seat{
			{"", "", "", "", "", "", "", ""},
			{"", "", "", "", "", "", "", ""},
			{"", "", "", "", "", "", "", ""},
			{"", "", "", "white", "black", "", "", ""},
			{"", "", "", "black", "white", "", "", ""},
			{"", "", "", "", "", "", "", ""},
			{"", "", "", "", "", "", "", ""},
			{"", "", "", "", "", "", "", ""},
		},
		LegalCells: cells("d3", "c4", "f5", "e6"),
		Discs:      DiscCount{Black: 2, White: 2},
	}
	if diff := cmp.Diff(want, st.View(discs.SeatBlack)); diff != "" {
		t.Errorf("black's view (-want +got):\n%s", diff)
	}
	if got := st.View(discs.SeatWhite).LegalCells; len(got) != 0 {
		t.Errorf("white, not to move, has legal cells %v", got)
	}
}

func TestApplyRejects(t *testing.T) {
	tests := []struct {
		name    string
		seat    game.Seat
		action  discs.Action
		wantErr error
	}{
		{"a taken cell", discs.SeatBlack, place("d4"), game.IllegalActionError{Msg: "d4 is taken"}},
		{"a cell that flips nothing", discs.SeatBlack, place("a1"), game.IllegalActionError{Msg: "a1 flips no disc; a move must flip at least one"}},
		{"a cell next to its own disc only", discs.SeatBlack, place("f4"), game.IllegalActionError{Msg: "f4 flips no disc; a move must flip at least one"}},
		{"column past h", discs.SeatBlack, place("i1"), game.IllegalActionError{Msg: `cell must be a1 to h8, a letter then a number, got "i1"`}},
		{"row 9", discs.SeatBlack, place("a9"), game.IllegalActionError{Msg: `cell must be a1 to h8, a letter then a number, got "a9"`}},
		{"unknown kind", discs.SeatBlack, discs.Action{Kind: "pass"}, game.IllegalActionError{Msg: "unknown action: pass"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := newGame(t).Apply(tc.seat, tc.action, nil)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestPlaceFlipsEveryClosedRun(t *testing.T) {
	before := position(discs.SeatBlack,
		"B..B..B.",
		".W.W.W..",
		"..WWW...",
		"BWW.WWWB",
		"..WWW...",
		".W.W.W..",
		"B..W....",
		"...B....",
	)
	next, events, err := before.Apply(discs.SeatBlack, place("d4"), nil)
	if err != nil {
		t.Fatal(err)
	}
	wantFlipped := cells("c3", "b2", "d3", "d2", "e3", "f2", "c4", "b4", "e4", "f4", "g4", "c5", "b6", "d5", "d6", "d7")
	if diff := cmp.Diff([]game.Event{PlacedEvent{By: discs.SeatBlack, Cell: cells("d4")[0], Flipped: wantFlipped}}, events); diff != "" {
		t.Errorf("events (-want +got):\n%s", diff)
	}
	view := next.View(discs.SeatWhite)
	if view.Discs != (DiscCount{Black: 24, White: 2}) {
		t.Errorf("discs %+v, want 24 black and the unclosed e5 and f6 white", view.Discs)
	}
	if view.Cells[4][4] != discs.SeatWhite || view.Cells[5][5] != discs.SeatWhite {
		t.Error("e5 and f6 flipped without a black disc behind them")
	}
}

func TestBlockedSeatPassesAutomatically(t *testing.T) {
	// After black takes a1, white has no move but black still has h8's neighbours.
	before := position(discs.SeatBlack,
		".WB.....",
		"........",
		"........",
		"........",
		"........",
		"........",
		"......WW",
		"......WB",
	)
	next, events, err := before.Apply(discs.SeatBlack, place("a1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []game.Event{
		PlacedEvent{By: discs.SeatBlack, Cell: cells("a1")[0], Flipped: cells("b1")},
		PassedEvent{By: discs.SeatWhite},
	}
	if diff := cmp.Diff(want, events); diff != "" {
		t.Errorf("events (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]game.Seat{discs.SeatBlack}, next.ToAct()); diff != "" {
		t.Errorf("to act (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(cells("f6", "h6", "f8"), next.View(discs.SeatBlack).LegalCells); diff != "" {
		t.Errorf("black's legal cells (-want +got):\n%s", diff)
	}
}

func TestGameEndsWhenNeitherSeatCanMove(t *testing.T) {
	tests := []struct {
		name  string
		state state
		cell  string
		want  game.Result
		discs DiscCount
	}{
		{
			name: "wipeout",
			state: position(discs.SeatBlack,
				".WB.....",
			),
			cell:  "a1",
			want:  game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{discs.SeatBlack}, Reason: ReasonMoreDiscs},
			discs: DiscCount{Black: 3},
		},
		{
			name: "full board, white ahead",
			state: position(discs.SeatBlack,
				".WBWWWWW", "BBWWWWWW", "WWWWWWWW", "WWWWWWWW",
				"WWWWWWWW", "BBBBBBBB", "BBBBBBBB", "BBBBBBBB",
			),
			cell:  "a1",
			want:  game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{discs.SeatWhite}, Reason: ReasonMoreDiscs},
			discs: DiscCount{Black: 29, White: 35},
		},
		{
			name: "full board, equal",
			state: position(discs.SeatBlack,
				".WBWWWWW", "BBWWWWWW", "WWWWWWWB", "WWWWWWWB",
				"WWWWWWWB", "BBBBBBBB", "BBBBBBBB", "BBBBBBBB",
			),
			cell:  "a1",
			want:  game.Result{Kind: game.ResultDraw, Reason: ReasonEqualDiscs},
			discs: DiscCount{Black: 32, White: 32},
		},
		{
			name: "both blocked with cells left",
			state: position(discs.SeatWhite,
				"WBBBBBB.", "........", "........", "........",
				"........", "........", "........", ".......B",
			),
			cell:  "h1",
			want:  game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{discs.SeatWhite}, Reason: ReasonMoreDiscs},
			discs: DiscCount{Black: 1, White: 8},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			next, events, err := tc.state.Apply(tc.state.ToAct()[0], place(tc.cell), nil)
			if err != nil {
				t.Fatal(err)
			}
			got, finished := next.Outcome()
			if diff := cmp.Diff(tc.want, got); !finished || diff != "" {
				t.Errorf("finished %t, result (-want +got):\n%s", finished, diff)
			}
			if len(events) != 1 {
				t.Errorf("events %v, want the place alone", events)
			}
			if view := next.View(discs.SeatBlack); view.Discs != tc.discs || len(view.LegalCells) > 0 {
				t.Errorf("discs %+v, legal %v; want %+v and none", view.Discs, view.LegalCells, tc.discs)
			}
		})
	}
}

func TestWipeoutFromTheStart(t *testing.T) {
	got, finished := play(t, wipeout()...).Outcome()
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{discs.SeatBlack}, Reason: ReasonMoreDiscs}
	if diff := cmp.Diff(want, got); !finished || diff != "" {
		t.Errorf("finished %t, result (-want +got):\n%s", finished, diff)
	}
}

func TestFullGamePassesAndEndsOnTheCount(t *testing.T) {
	st := newGame(t)
	passes := 0
	for _, cell := range firstLegalGame(t) {
		next, events, err := st.Apply(st.ToAct()[0], place(cell), nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range events[1:] {
			if e != (PassedEvent{By: discs.SeatBlack}) {
				t.Errorf("after %s: event %+v, want black's pass", cell, e)
			}
			passes++
		}
		st = next
	}
	if passes != 4 {
		t.Errorf("%d passes, want 4", passes)
	}
	if got := st.View(discs.SeatBlack).Discs; got != (DiscCount{Black: 19, White: 45}) {
		t.Errorf("discs %+v", got)
	}
}

func TestApplyLeavesEarlierStateUnchanged(t *testing.T) {
	before := play(t, "d3", "c3")
	want := before.View(discs.SeatBlack)
	if _, _, err := before.Apply(discs.SeatBlack, place("b3"), nil); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, before.View(discs.SeatBlack)); diff != "" {
		t.Errorf("earlier state changed (-before +after):\n%s", diff)
	}
}

func TestLegalCellsClearOnAnImposedResult(t *testing.T) {
	imposed := game.Result{Kind: game.ResultDraw, Reason: game.ReasonDrawAgreed}
	concluded := newGame(t).Conclude(imposed)
	if got := concluded.View(discs.SeatBlack).LegalCells; len(got) != 0 {
		t.Errorf("legal cells %v after an imposed result", got)
	}
	if got, _ := concluded.Outcome(); !cmp.Equal(imposed, got) {
		t.Errorf("result %+v", got)
	}
	won := play(t, wipeout()...)
	if got, _ := won.Conclude(imposed).Outcome(); got.Reason != ReasonMoreDiscs {
		t.Errorf("a reached result was replaced by %+v", got)
	}
}

func TestWireShape(t *testing.T) {
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(json.RawMessage(`{"first":"me"}`))
	if err != nil {
		t.Fatal(err)
	}
	st, _, err := g.New(opts, alice, []game.UserID{bob}, game.StepRNG(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	st, events, err := st.Apply(discs.SeatBlack, json.RawMessage(`{"kind":"place","cell":"d3"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || string(events[0]) != `{"kind":"placed","by":"black","cell":{"row":3,"col":4},"flipped":[{"row":4,"col":4}]}` {
		t.Errorf("events %s", events)
	}
	view, err := st.View(discs.SeatWhite)
	if err != nil {
		t.Fatal(err)
	}
	empty := `["","","","","","","",""]`
	want := `{"cells":[` + empty + `,` + empty + `,["","","","black","","","",""],["","","","black","black","","",""],["","","","black","white","","",""],` +
		empty + `,` + empty + `,` + empty + `],"last":{"row":3,"col":4},` +
		`"legal_cells":[{"row":3,"col":3},{"row":3,"col":5},{"row":5,"col":3}],"discs":{"black":4,"white":1}}`
	if string(view) != want {
		t.Errorf("view\n%s\nwant\n%s", view, want)
	}
}

func TestCataloguePatternIsTheParsersOwn(t *testing.T) {
	if got := (Rules{}).Info().Actions[0].Input.Pattern; got != coords().Pattern() {
		t.Errorf("catalogue pattern %s, want %s", got, coords().Pattern())
	}
}

func TestConformance(t *testing.T) {
	script := func(name string, steps []gametest.Step[discs.Action], want *game.Result) gametest.Script[game.TurnOpts, discs.Action] {
		return gametest.Script[game.TurnOpts, discs.Action]{
			Name:       name,
			Opts:       game.TurnOpts{First: game.FirstMoverRandom, TimeControl: game.TimeControlNone},
			Seed:       1,
			Steps:      steps,
			WantResult: want,
		}
	}
	flipsNothing := gametest.Step[discs.Action]{Seat: discs.SeatWhite, Action: place("a1"), WantErr: true}
	gametest.Run(t, Rules{}, []gametest.Script[game.TurnOpts, discs.Action]{
		script("wipeout after a rejected move", slices.Insert(steps(t, wipeout()...), 1, flipsNothing),
			&game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{discs.SeatBlack}, Reason: ReasonMoreDiscs}),
		script("a full game with passes", steps(t, firstLegalGame(t)...),
			&game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{discs.SeatWhite}, Reason: ReasonMoreDiscs}),
		script("ongoing", steps(t, "d3", "c3"), nil),
	})
}
