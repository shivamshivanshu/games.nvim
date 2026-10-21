package battleship

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func TestNewSeatsTheFirstMoverAsP1(t *testing.T) {
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
			st, seats, err := Rules{}.New(Opts{First: tc.first}, alice, []game.UserID{bob}, game.StepRNG(tc.seed, 0))
			if err != nil {
				t.Fatal(err)
			}
			if seats[alice] != tc.wantAlice || len(seats) != 2 || seats[bob] == tc.wantAlice {
				t.Errorf("seats %v, want alice %s", seats, tc.wantAlice)
			}
			if diff := cmp.Diff([]game.Seat{p1, p2}, st.ToAct()); diff != "" {
				t.Errorf("both seats place at once (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewNeedsExactlyOneOpponent(t *testing.T) {
	for _, invitees := range [][]game.UserID{nil, {bob, "carol"}} {
		_, _, err := Rules{}.New(Opts{}, alice, invitees, game.StepRNG(1, 0))
		want := game.BadRequestError{Msg: fmt.Sprintf("battleship needs exactly one opponent, got %d", len(invitees))}
		if !errors.Is(err, want) {
			t.Errorf("%d invitees: got %v", len(invitees), err)
		}
	}
}

func TestApplyRejects(t *testing.T) {
	carrierAtA1 := moves(p1, across("a1"))
	p1Ready := moves(p1, rowsFleet()...)
	tests := []struct {
		name   string
		before []move
		seat   game.Seat
		action Action
		want   error
	}{
		{"off the board across", nil, p1, across("g1"), game.IllegalActionError{Msg: "the carrier across from g1 runs off the board"}},
		{"off the board down", nil, p2, down("a7"), game.IllegalActionError{Msg: "the carrier down from a7 runs off the board"}},
		{"overlap", carrierAtA1, p1, down("c1"), game.IllegalActionError{Msg: "the battleship down from c1 overlaps your carrier"}},
		{"bad cell", nil, p1, across("k1"), game.IllegalActionError{Msg: `cell must be a1 to j10, a letter then a number, got "k1"`}},
		{"row 11", nil, p1, down("a11"), game.IllegalActionError{Msg: `cell must be a1 to j10, a letter then a number, got "a11"`}},
		{"fire during setup", nil, p1, fire("a1"), game.IllegalActionError{Msg: "fire once both fleets are placed"}},
		{"a placed fleet waits", p1Ready, p1, across("a1"), game.ErrNotYourTurn},
		{"unknown kind", nil, p2, Action{Kind: "salvo", Cell: "a1"}, game.IllegalActionError{Msg: "unknown action: salvo"}},
		{"a seat not in the game", nil, "p3", random(), game.ErrNotYourTurn},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := playFrom(t, newGame(t), 1, tc.before...)
			_, _, err := st.Apply(tc.seat, tc.action, game.StepRNG(1, 99))
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestBattleRejects(t *testing.T) {
	tests := []struct {
		name   string
		before []move
		seat   game.Seat
		action Action
		want   error
	}{
		{"out of turn", nil, p2, fire("a1"), game.ErrNotYourTurn},
		{"place in battle", nil, p1, across("j1"), game.IllegalActionError{Msg: "the fleets are placed; fire at a cell"}},
		{"random in battle", nil, p1, random(), game.IllegalActionError{Msg: "the fleets are placed; fire at a cell"}},
		{"twice at one cell", []move{{p1, fire("j10")}, {p2, fire("j10")}}, p1, fire(" J10 "), game.IllegalActionError{Msg: "you have already fired at j10"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := playFrom(t, placed(t), 1, tc.before...)
			_, _, err := st.Apply(tc.seat, tc.action, game.StepRNG(1, 99))
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSetupIsSimultaneousAndTheSecondFleetStartsTheBattle(t *testing.T) {
	st := newGame(t)
	for i, a := range rowsFleet() {
		next, events, err := st.Apply(p1, a, nil)
		if err != nil {
			t.Fatal(err)
		}
		wantEvents := []game.Event{}
		if i == len(rowsFleet())-1 {
			wantEvents = []game.Event{ReadyEvent{By: p1}}
		}
		if diff := cmp.Diff(wantEvents, events); diff != "" {
			t.Errorf("ship %d events (-want +got):\n%s", i, diff)
		}
		st = next
	}
	if diff := cmp.Diff([]game.Seat{p2}, st.ToAct()); diff != "" {
		t.Errorf("after p1's fleet (-want +got):\n%s", diff)
	}
	if got := st.View(p1).Phase; got != PhaseSetup {
		t.Errorf("phase %s while p2 places", got)
	}
	st, events, err := st.Apply(p2, random(), game.StepRNG(1, 6))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]game.Event{ReadyEvent{By: p2}}, events); diff != "" {
		t.Errorf("random fill events (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]game.Seat{p1}, st.ToAct()); diff != "" {
		t.Errorf("battle starts with p1 (-want +got):\n%s", diff)
	}
	if st.View(p2).Phase != PhaseBattle {
		t.Errorf("phase %s", st.View(p2).Phase)
	}
}

func TestSetupViewOffersOnlyTheViewersNextShipAndItsAnchors(t *testing.T) {
	st := playFrom(t, newGame(t), 1, moves(p1, across("a1"))...)
	placing := st.View(p1)
	if diff := cmp.Diff(&ShipSpec{Name: ShipBattleship, Size: 4}, placing.Next); diff != "" {
		t.Errorf("next (-want +got):\n%s", diff)
	}
	// A battleship across fits 7 anchors a row; on row 1 the carrier leaves it f1 and g1.
	if got := len(placing.Anchors.Across); got != 9*7+2 {
		t.Errorf("%d across anchors", got)
	}
	// Down it fits 7 anchors a column; columns a to e lose row 1.
	if got := len(placing.Anchors.Down); got != 10*7-5 {
		t.Errorf("%d down anchors", got)
	}
	if slices.Contains(placing.Anchors.Across, Square{Row: 1, Col: 2}) || !slices.Contains(placing.Anchors.Across, Square{Row: 1, Col: 6}) {
		t.Errorf("across anchors on row 1: %v", placing.Anchors.Across[:3])
	}
	if diff := cmp.Diff([]ShipState{ShipAfloat, ShipUnplaced, ShipUnplaced, ShipUnplaced, ShipUnplaced}, states(placing.Fleet)); diff != "" {
		t.Errorf("own fleet (-want +got):\n%s", diff)
	}
	if got := placing.Own[0][:6]; !cmp.Equal(got, []Tile{TileShip, TileShip, TileShip, TileShip, TileShip, TileEmpty}) {
		t.Errorf("own row 1 %v", got)
	}
	other := st.View(p2)
	if diff := cmp.Diff(&ShipSpec{Name: ShipCarrier, Size: 5}, other.Next); diff != "" {
		t.Errorf("p2 next (-want +got):\n%s", diff)
	}
	if other.Target[0][0] != TileEmpty || slices.Contains(states(other.Enemy), ShipUnplaced) {
		t.Errorf("p2 sees p1's placement: target %v enemy %v", other.Target[0], other.Enemy)
	}
	waiting := playFrom(t, st, 1, moves(p1, rowsFleet()[1:]...)...).View(p1)
	if waiting.Next != nil || len(waiting.Anchors.Across)+len(waiting.Anchors.Down)+len(waiting.Targets) != 0 {
		t.Errorf("a ready seat is offered %+v %+v %v", waiting.Next, waiting.Anchors, waiting.Targets)
	}
}

func states(fleet []ShipStatus) []ShipState {
	out := make([]ShipState, len(fleet))
	for i, s := range fleet {
		out[i] = s.State
	}
	return out
}

func TestRandomFillsTheRestOfTheFleetLegallyFromTheStepRNG(t *testing.T) {
	layouts := map[string]bool{}
	for seed := game.Seed(1); seed <= 20; seed++ {
		st := playFrom(t, newGame(t), seed, moves(p1, across("a1"), random())...)
		cells := shipNames(st, p1)
		if len(cells) != 17 {
			t.Fatalf("seed %d: %d ship cells", seed, len(cells))
		}
		if diff := cmp.Diff([]string{"a1", "b1", "c1", "d1", "e1"}, cells[:5]); diff != "" {
			t.Errorf("seed %d: the carrier moved (-want +got):\n%s", seed, diff)
		}
		if distinct := len(sortedUnique(cells)); distinct != 17 {
			t.Errorf("seed %d: ships overlap, %d distinct cells of %v", seed, distinct, cells)
		}
		again := playFrom(t, newGame(t), seed, moves(p1, across("a1"), random())...)
		if !cmp.Equal(cells, shipNames(again, p1)) {
			t.Errorf("seed %d: the same seed placed differently", seed)
		}
		if !slices.Equal(st.ToAct(), []game.Seat{p2}) {
			t.Errorf("seed %d: to act %v", seed, st.ToAct())
		}
		layouts[fmt.Sprint(cells)] = true
	}
	if len(layouts) < 15 {
		t.Errorf("only %d layouts from 20 seeds", len(layouts))
	}
}

func sortedUnique(s []string) []string {
	return slices.Compact(slices.Sorted(slices.Values(s)))
}

func TestShotsReportMissHitAndTheShipTheySink(t *testing.T) {
	st := placed(t)
	shots := []struct {
		seat game.Seat
		cell string
		want ShotEvent
	}{
		{p1, "j10", ShotEvent{By: p1, Cell: Square{Row: 10, Col: 10}, Outcome: ShotMiss}},
		{p2, "a9", ShotEvent{By: p2, Cell: Square{Row: 9, Col: 1}, Outcome: ShotHit}},
		{p1, "i1", ShotEvent{By: p1, Cell: Square{Row: 1, Col: 9}, Outcome: ShotHit}},
		{p2, "b9", ShotEvent{By: p2, Cell: Square{Row: 9, Col: 2}, Outcome: ShotSunk, Ship: ShipDestroyer}},
		{p1, "I2", ShotEvent{By: p1, Cell: Square{Row: 2, Col: 9}, Outcome: ShotSunk, Ship: ShipDestroyer}},
	}
	for _, shot := range shots {
		next, events, err := st.Apply(shot.seat, fire(shot.cell), nil)
		if err != nil {
			t.Fatalf("%s fires %s: %v", shot.seat, shot.cell, err)
		}
		if diff := cmp.Diff([]game.Event{shot.want}, events); diff != "" {
			t.Errorf("%s fires %s (-want +got):\n%s", shot.seat, shot.cell, diff)
		}
		st = next
	}
	alice := st.View(p1)
	if diff := cmp.Diff([]Tile{TileEmpty, TileEmpty, TileSunk}, []Tile{alice.Target[0][7], alice.Target[0][9], alice.Target[0][8]}); diff != "" {
		t.Errorf("alice's target h1 j1 i1 (-want +got):\n%s", diff)
	}
	if alice.Target[9][9] != TileMiss || alice.Own[8][0] != TileSunk || alice.Own[0][0] != TileShip {
		t.Errorf("alice's j10 %s, own a9 %s, own a1 %s", alice.Target[9][9], alice.Own[8][0], alice.Own[0][0])
	}
	if diff := cmp.Diff([]ShipState{ShipAfloat, ShipAfloat, ShipAfloat, ShipAfloat, ShipSunk}, states(alice.Enemy)); diff != "" {
		t.Errorf("enemy fleet (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(&shots[4].want, alice.Last); diff != "" {
		t.Errorf("last (-want +got):\n%s", diff)
	}
	bob := st.View(p2)
	if len(bob.Targets) != 98 || slices.Contains(bob.Targets, Square{Row: 9, Col: 1}) || len(alice.Targets) != 0 {
		t.Errorf("bob may fire at %d cells, alice at %d", len(bob.Targets), len(alice.Targets))
	}
}

func TestSinkingTheLastShipWins(t *testing.T) {
	st := placed(t)
	bobsShips := shipNames(st, p2)
	misses := []string{"j1", "j2", "j3", "j4", "j5", "j6", "j7", "j8", "j9", "j10", "i10", "h10", "g10", "f10", "e10", "d10"}
	var ms []move
	for i, cell := range bobsShips {
		ms = append(ms, move{p1, fire(cell)})
		if i < len(bobsShips)-1 {
			ms = append(ms, move{p2, fire(misses[i])})
		}
	}
	st = playFrom(t, st, 1, ms...)
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{p1}, Reason: ReasonFleetSunk}
	if got, finished := st.Outcome(); !finished || !cmp.Equal(want, got) {
		t.Errorf("outcome %+v, %t", got, finished)
	}
	if len(st.ToAct()) != 0 || st.View(p1).Phase != PhaseOver {
		t.Errorf("to act %v, phase %s", st.ToAct(), st.View(p1).Phase)
	}
	if bob := st.View(p2); bob.Target[0][0] != TileShip || bob.Target[4][6] != TileEmpty {
		t.Errorf("the loser sees a1 %s and g5 %s on its target once over", bob.Target[0][0], bob.Target[4][6])
	}
}

func TestConcludeClearsOffersAndRevealsTheFleets(t *testing.T) {
	imposed := game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	setup := playFrom(t, newGame(t), 1, moves(p1, across("a1"))...)
	concluded := setup.(game.Concluder[Action, View]).Conclude(imposed)
	for _, seat := range []game.Seat{p1, p2} {
		v := concluded.View(seat)
		if v.Next != nil || len(v.Anchors.Across)+len(v.Anchors.Down)+len(v.Targets) != 0 || v.Phase != PhaseOver {
			t.Errorf("%s after an imposed end: %+v", seat, v)
		}
	}
	if got := concluded.View(p2).Target[0][4]; got != TileShip {
		t.Errorf("p2 sees p1's carrier end as %s once over", got)
	}
	battle := placed(t).(game.Concluder[Action, View]).Conclude(imposed)
	if v := battle.View(p1); len(v.Targets) != 0 {
		t.Errorf("targets %d after an imposed end", len(v.Targets))
	}
	resigned, events := placed(t).Eliminate(p1, game.ReasonResign)
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{p2}, Reason: game.ReasonResign}
	if got, finished := resigned.Outcome(); !finished || !cmp.Equal(want, got) || len(events) != 0 {
		t.Errorf("resign: %+v %t %v", got, finished, events)
	}
	if got, _ := resigned.(game.Concluder[Action, View]).Conclude(imposed).Outcome(); !cmp.Equal(want, got) {
		t.Errorf("a reached result was replaced by %+v", got)
	}
}

func TestApplyLeavesEarlierStateUnchanged(t *testing.T) {
	for _, m := range []move{{p1, across("a1")}, {p1, random()}} {
		before := newGame(t)
		want := before.View(p1)
		if _, _, err := before.Apply(m.seat, m.action, game.StepRNG(1, 1)); err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, before.View(p1)); diff != "" {
			t.Errorf("%+v changed the earlier state (-before +after):\n%s", m.action, diff)
		}
	}
	before := placed(t)
	want := before.View(p2)
	if _, _, err := before.Apply(p1, fire("a1"), nil); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, before.View(p2)); diff != "" {
		t.Errorf("a shot changed the earlier state (-before +after):\n%s", diff)
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
	if st, _, err = st.Apply(p1, json.RawMessage(`{"kind":"random"}`), game.StepRNG(1, 1)); err != nil {
		t.Fatal(err)
	}
	st, events, err := st.Apply(p2, json.RawMessage(`{"kind":"random"}`), game.StepRNG(1, 2))
	if err != nil || len(events) != 1 || string(events[0]) != `{"by":"p2","kind":"ready"}` {
		t.Fatalf("ready: %s, %v", events, err)
	}
	st, events, err = st.Apply(p1, json.RawMessage(`{"kind":"fire","cell":"a1"}`), nil)
	if err != nil || len(events) != 1 {
		t.Fatalf("fire: %s, %v", events, err)
	}
	var shot map[string]any
	if err := json.Unmarshal(events[0], &shot); err != nil {
		t.Fatal(err)
	}
	if shot["kind"] != "shot" || shot["by"] != "p1" || shot["cell"] == nil || shot["outcome"] == nil {
		t.Errorf("shot %s", events[0])
	}
	raw, err := st.View(p1)
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]json.RawMessage
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	for field, want := range map[string]string{"phase": `"battle"`, "next": "null", "anchors": `{"across":[],"down":[]}`, "targets": "[]"} {
		if got := string(view[field]); got != want {
			t.Errorf("%s = %s, want %s", field, got, want)
		}
	}
}

func TestCataloguePatternIsTheParsersOwn(t *testing.T) {
	for _, spec := range (Rules{}).Info().Actions {
		if spec.Input != nil && spec.Input.Pattern != coords().Pattern() {
			t.Errorf("%s pattern %s, want %s", spec.Kind, spec.Input.Pattern, coords().Pattern())
		}
	}
}
