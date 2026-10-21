package dotsandboxes

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
	p1    game.Seat   = "p1"
	p2    game.Seat   = "p2"
)

func draw(edge string) Action { return Action{Kind: ActionDraw, Edge: edge} }

// newGame has alice as p1, moving first, and bob as p2.
func newGame(t *testing.T, size Size) state {
	t.Helper()
	st, seats, err := Rules{}.New(Opts{Size: size, First: game.FirstMoverMe}, alice, []game.UserID{bob}, game.StepRNG(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if seats[alice] != p1 || seats[bob] != p2 {
		t.Fatalf("seats %v", seats)
	}
	return st.(state)
}

type move struct {
	seat game.Seat
	edge string
}

// played applies each move in turn, failing on any refusal; it returns the last move's events.
func played(t *testing.T, s state, moves ...move) (state, []game.Event) {
	t.Helper()
	var events []game.Event
	for i, m := range moves {
		next, evs, err := s.Apply(m.seat, draw(m.edge), nil)
		if err != nil {
			t.Fatalf("move %d, %s %s: %v", i, m.seat, m.edge, err)
		}
		s, events = next.(state), evs
	}
	return s, events
}

// playedOut has the mover draw the edge choose picks from its legal edges until the game ends.
func playedOut(t *testing.T, size Size, choose func(legal []Line) Line) ([]move, state) {
	t.Helper()
	s := newGame(t, size)
	var moves []move
	for !s.finished {
		mover := s.toMove
		line := choose(s.View(mover).LegalEdges)
		m := move{seat: mover, edge: nameOf(line)}
		s, _ = played(t, s, m)
		moves = append(moves, m)
	}
	return moves, s
}

func first(legal []Line) Line { return legal[0] }
func last(legal []Line) Line  { return legal[len(legal)-1] }

func nameOf(line Line) string {
	return catalogueEdges().Name(grid.Edge{Dot: grid.Point{Row: line.Row - 1, Col: line.Col - 1}, Orientation: line.Orientation})
}

// boxA1 draws every side of the top left box but its right one, b1v, leaving p2 to move.
func boxA1() []move { return []move{{p1, "a1h"}, {p2, "a2h"}, {p1, "a1v"}} }

func TestNewBoardOfEachSize(t *testing.T) {
	for _, tc := range []struct {
		size Size
		side int
	}{{Size3, 3}, {Size5, 5}, {Size7, 7}} {
		t.Run(string(tc.size), func(t *testing.T) {
			s := newGame(t, tc.size)
			mover, waiting := s.View(p1), s.View(p2)
			if got := [6]int{len(mover.Across), len(mover.Across[0]), len(mover.Down), len(mover.Down[0]), len(mover.Boxes), len(mover.Boxes[0])}; got != [6]int{tc.side + 1, tc.side, tc.side, tc.side + 1, tc.side, tc.side} {
				t.Errorf("across, down and boxes rows by columns = %v", got)
			}
			if want := 2 * tc.side * (tc.side + 1); len(mover.LegalEdges) != want {
				t.Errorf("p1 has %d legal edges, want %d", len(mover.LegalEdges), want)
			}
			if len(waiting.LegalEdges) != 0 || waiting.LegalEdges == nil {
				t.Errorf("p2 legal edges %v, want []", waiting.LegalEdges)
			}
			if diff := cmp.Diff(map[game.Seat]int{p1: 0, p2: 0}, mover.Scores); diff != "" || mover.Last != nil {
				t.Errorf("scores (-want +got):\n%s, last %v", diff, mover.Last)
			}
			if diff := cmp.Diff([]game.Seat{p1}, s.ToAct()); diff != "" {
				t.Errorf("to act (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFirstMoverIsP1(t *testing.T) {
	tests := []struct {
		first     game.FirstMover
		seed      game.Seed
		wantAlice game.Seat
	}{
		{game.FirstMoverMe, 1, p1},
		{game.FirstMoverOpponent, 1, p2},
		{game.FirstMoverRandom, 1, p2},
		{game.FirstMoverRandom, 6, p1},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s seed %d", tc.first, tc.seed), func(t *testing.T) {
			_, seats, err := Rules{}.New(Opts{Size: Size3, First: tc.first}, alice, []game.UserID{bob}, game.StepRNG(tc.seed, 0))
			if err != nil {
				t.Fatal(err)
			}
			wantBob := p1
			if tc.wantAlice == p1 {
				wantBob = p2
			}
			if seats[alice] != tc.wantAlice || seats[bob] != wantBob {
				t.Errorf("seats %v, want alice %s", seats, tc.wantAlice)
			}
		})
	}
}

func TestNewRefuses(t *testing.T) {
	tests := []struct {
		name     string
		opts     Opts
		invitees []game.UserID
		want     error
	}{
		{"no opponent", Opts{Size: Size3}, nil, game.BadRequestError{Msg: "dots_and_boxes needs exactly one opponent, got 0"}},
		{"two opponents", Opts{Size: Size3}, []game.UserID{bob, "carol"}, game.BadRequestError{Msg: "dots_and_boxes needs exactly one opponent, got 2"}},
		{"unknown size", Opts{Size: "4"}, []game.UserID{bob}, game.BadRequestError{Msg: `unknown size "4"`}},
	}
	for _, tc := range tests {
		_, _, err := Rules{}.New(tc.opts, alice, tc.invitees, game.StepRNG(1, 0))
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, err, tc.want)
		}
	}
}

func TestDrawing(t *testing.T) {
	tests := []struct {
		name       string
		moves      []move
		wantEvent  DrawnEvent
		wantToAct  game.Seat
		wantBoxes  map[game.Seat][]Box
		wantScores map[game.Seat]int
	}{
		{
			name:       "an edge that completes no box passes the move",
			moves:      []move{{p1, "b2v"}},
			wantEvent:  DrawnEvent{By: p1, Edge: Line{Row: 2, Col: 2, Orientation: grid.EdgeDown}, Boxes: []Box{}},
			wantToAct:  p2,
			wantScores: map[game.Seat]int{p1: 0, p2: 0},
		},
		{
			name:       "completing a box claims it and moves again",
			moves:      append(boxA1(), move{p2, "b1v"}),
			wantEvent:  DrawnEvent{By: p2, Edge: Line{Row: 1, Col: 2, Orientation: grid.EdgeDown}, Boxes: []Box{{Row: 1, Col: 1}}},
			wantToAct:  p2,
			wantBoxes:  map[game.Seat][]Box{p2: {{Row: 1, Col: 1}}},
			wantScores: map[game.Seat]int{p1: 0, p2: 1},
		},
		{
			name: "one edge completes the boxes either side of it",
			moves: []move{
				{p1, "a1h"}, {p2, "a2h"}, {p1, "a1v"},
				{p2, "b1h"}, {p1, "b2h"}, {p2, "c1v"},
				{p1, "b1v"},
			},
			wantEvent:  DrawnEvent{By: p1, Edge: Line{Row: 1, Col: 2, Orientation: grid.EdgeDown}, Boxes: []Box{{Row: 1, Col: 1}, {Row: 1, Col: 2}}},
			wantToAct:  p1,
			wantBoxes:  map[game.Seat][]Box{p1: {{Row: 1, Col: 1}, {Row: 1, Col: 2}}},
			wantScores: map[game.Seat]int{p1: 2, p2: 0},
		},
		{
			name:       "an edge closing no box beside a box still open passes the move",
			moves:      append(boxA1(), move{p2, "c1v"}),
			wantEvent:  DrawnEvent{By: p2, Edge: Line{Row: 1, Col: 3, Orientation: grid.EdgeDown}, Boxes: []Box{}},
			wantToAct:  p1,
			wantScores: map[game.Seat]int{p1: 0, p2: 0},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, events := played(t, newGame(t, Size3), tc.moves...)
			if diff := cmp.Diff([]game.Event{tc.wantEvent}, events); diff != "" {
				t.Errorf("events (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff([]game.Seat{tc.wantToAct}, s.ToAct()); diff != "" {
				t.Errorf("to act (-want +got):\n%s", diff)
			}
			view := s.View(tc.wantToAct)
			if diff := cmp.Diff(tc.wantScores, view.Scores); diff != "" {
				t.Errorf("scores (-want +got):\n%s", diff)
			}
			for r, row := range view.Boxes {
				for c, owner := range row {
					box := Box{Row: r + 1, Col: c + 1}
					if want := ownerIn(tc.wantBoxes, box); owner != want {
						t.Errorf("box %+v owned by %q, want %q", box, owner, want)
					}
				}
			}
			wantLast := tc.wantEvent.Edge
			if view.Last == nil || *view.Last != wantLast {
				t.Errorf("last %v, want %+v", view.Last, wantLast)
			}
			if want := 24 - len(tc.moves); len(view.LegalEdges) != want {
				t.Errorf("%d legal edges, want %d", len(view.LegalEdges), want)
			}
		})
	}
}

func ownerIn(boxes map[game.Seat][]Box, box Box) game.Seat {
	for seat, owned := range boxes {
		for _, b := range owned {
			if b == box {
				return seat
			}
		}
	}
	return ""
}

func TestTheViewRecordsWhoDrewEachEdge(t *testing.T) {
	s, _ := played(t, newGame(t, Size3), append(boxA1(), move{p2, "b1v"}, move{p2, "d3v"})...)
	view := s.View(p1)
	want := map[string]game.Seat{"a1h": p1, "a2h": p2, "a1v": p1, "b1v": p2, "d3v": p2}
	for r, row := range view.Across {
		for c, by := range row {
			name := nameOf(Line{Row: r + 1, Col: c + 1, Orientation: grid.EdgeAcross})
			if by != want[name] {
				t.Errorf("%s drawn by %q, want %q", name, by, want[name])
			}
		}
	}
	for r, row := range view.Down {
		for c, by := range row {
			name := nameOf(Line{Row: r + 1, Col: c + 1, Orientation: grid.EdgeDown})
			if by != want[name] {
				t.Errorf("%s drawn by %q, want %q", name, by, want[name])
			}
		}
	}
}

func TestRefusals(t *testing.T) {
	_, finished := playedOut(t, Size3, first)
	opened, _ := played(t, newGame(t, Size3), move{p1, "a1h"})
	tests := []struct {
		name   string
		state  state
		seat   game.Seat
		action Action
		want   error
	}{
		{"out of turn", newGame(t, Size3), p2, draw("a1h"), game.ErrNotYourTurn},
		{"after the end", finished, p1, draw("a1h"), game.ErrNotYourTurn},
		{"unknown kind", newGame(t, Size3), p1, Action{Kind: "fill", Edge: "a1h"}, game.IllegalActionError{Msg: "unknown action: fill"}},
		{"no orientation", newGame(t, Size3), p1, draw("b2"), game.IllegalActionError{Msg: `edge must be a dot then h or v, such as b2h, got "b2"`}},
		{"off the largest board", newGame(t, Size3), p1, draw("i1h"), game.IllegalActionError{Msg: `edge must be a dot then h or v, such as b2h, got "i1h"`}},
		{"across from the last column", newGame(t, Size3), p1, draw("d1h"), game.IllegalActionError{Msg: "d1h is off the board of 3 boxes a side"}},
		{"down from the last row", newGame(t, Size5), p1, draw("A6V"), game.IllegalActionError{Msg: "a6v is off the board of 5 boxes a side"}},
		{"already drawn", opened, p2, draw(" A1H "), game.IllegalActionError{Msg: "a1h is already drawn"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tc.state.Apply(tc.seat, tc.action, nil)
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestApplyLeavesTheEarlierStateUnchanged(t *testing.T) {
	before, _ := played(t, newGame(t, Size3), boxA1()...)
	views := map[game.Seat]View{p1: before.View(p1), p2: before.View(p2)}
	if _, _, err := before.Apply(p2, draw("b1v"), nil); err != nil {
		t.Fatal(err)
	}
	for seat, view := range views {
		if diff := cmp.Diff(view, before.View(seat)); diff != "" {
			t.Errorf("%s view changed (-before +after):\n%s", seat, diff)
		}
	}
	if diff := cmp.Diff([]game.Seat{p2}, before.ToAct()); diff != "" {
		t.Errorf("to act changed (-want +got):\n%s", diff)
	}
}

// The scores were checked against an independent simulation of the same choices.
func TestTheLastBoxEndsTheGameAndTheMostBoxesWin(t *testing.T) {
	p2Wins := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{p2}, Reason: ReasonMoreBoxes}
	for _, tc := range []struct {
		size   Size
		choose func([]Line) Line
		want   game.Result
		scores map[game.Seat]int
	}{
		{Size3, first, p2Wins, map[game.Seat]int{p1: 2, p2: 7}},
		{Size3, last, p2Wins, map[game.Seat]int{p1: 3, p2: 6}},
		{Size5, first, p2Wins, map[game.Seat]int{p1: 8, p2: 17}},
		{Size7, first, p2Wins, map[game.Seat]int{p1: 18, p2: 31}},
	} {
		moves, s := playedOut(t, tc.size, tc.choose)
		side := boxesPerSide(tc.size)
		if want := 2 * side * (side + 1); len(moves) != want {
			t.Errorf("%s: %d moves, want one per edge, %d", tc.size, len(moves), want)
		}
		result, finished := s.Outcome()
		if !finished || !cmp.Equal(tc.want, result) {
			t.Errorf("%s: outcome %+v, %t; want %+v", tc.size, result, finished, tc.want)
		}
		view := s.View(p1)
		if diff := cmp.Diff(tc.scores, view.Scores); diff != "" {
			t.Errorf("%s: scores (-want +got):\n%s", tc.size, diff)
		}
		if len(s.ToAct()) != 0 || len(view.LegalEdges) != 0 {
			t.Errorf("%s: to act %v, legal %v after the end", tc.size, s.ToAct(), view.LegalEdges)
		}
	}
}

func TestEqualBoxesDraw(t *testing.T) {
	even := newState(2, []game.Seat{p1, p2})
	even.owners = map[grid.Point]game.Seat{{Row: 0, Col: 0}: p1, {Row: 0, Col: 1}: p2, {Row: 1, Col: 0}: p2, {Row: 1, Col: 1}: p1}
	if got, want := even.ended().result, (game.Result{Kind: game.ResultDraw, Reason: ReasonEqualBoxes}); !cmp.Equal(want, got) {
		t.Errorf("2 boxes each: %+v, want %+v", got, want)
	}
	even.owners[grid.Point{Row: 1, Col: 1}] = p2
	if got, want := even.ended().result, (game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{p2}, Reason: ReasonMoreBoxes}); !cmp.Equal(want, got) {
		t.Errorf("3 to 1: %+v, want %+v", got, want)
	}
}

func TestConcludeOffersNoEdgeAndKeepsAReachedResult(t *testing.T) {
	agreed := game.Result{Kind: game.ResultDraw, Reason: game.ReasonDrawAgreed}
	concluded := newGame(t, Size3).Conclude(agreed)
	if result, finished := concluded.Outcome(); !finished || !cmp.Equal(agreed, result) {
		t.Errorf("outcome %+v, %t", result, finished)
	}
	if view := concluded.View(p1); len(concluded.ToAct()) != 0 || len(view.LegalEdges) != 0 || view.LegalEdges == nil {
		t.Errorf("to act %v, legal %v after a conclusion", concluded.ToAct(), view.LegalEdges)
	}
	_, finished := playedOut(t, Size3, first)
	reached, _ := finished.Outcome()
	if result, _ := finished.Conclude(agreed).Outcome(); !cmp.Equal(reached, result) {
		t.Errorf("conclusion replaced %+v with %+v", reached, result)
	}
	resigned, _ := newGame(t, Size3).Eliminate(p1, game.ReasonResign)
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{p2}, Reason: game.ReasonResign}
	if result, _ := resigned.Outcome(); !cmp.Equal(want, result) {
		t.Errorf("resign: %+v, want %+v", result, want)
	}
}

func TestCatalogue(t *testing.T) {
	info := Rules{}.Info()
	if err := info.CheckActions(); err != nil {
		t.Fatal(err)
	}
	if err := info.CheckOptions(); err != nil {
		t.Fatal(err)
	}
	if got, want := info.Actions[0].Input.Pattern, grid.NewEdges(7, 7).Pattern(); got != want {
		t.Errorf("pattern %s, want the 7-box board's %s", got, want)
	}
}
