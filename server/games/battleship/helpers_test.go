package battleship

import (
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
	p1    game.Seat   = "p1"
	p2    game.Seat   = "p2"
)

func across(cell string) Action { return Action{Kind: ActionPlaceAcross, Cell: cell} }
func down(cell string) Action   { return Action{Kind: ActionPlaceDown, Cell: cell} }
func random() Action            { return Action{Kind: ActionRandom} }
func fire(cell string) Action   { return Action{Kind: ActionFire, Cell: cell} }

// rowsFleet lays every ship across from column a: carrier a1-e1, battleship
// a3-d3, cruiser a5-c5, submarine a7-c7, destroyer a9-b9.
func rowsFleet() []Action {
	return []Action{across("a1"), across("a3"), across("a5"), across("a7"), across("a9")}
}

// columnsFleet lays every ship down from row 1: carrier a1-a5, battleship
// c1-c4, cruiser e1-e3, submarine g1-g3, destroyer i1-i2.
func columnsFleet() []Action {
	return []Action{down("a1"), down("c1"), down("e1"), down("g1"), down("i1")}
}

// newGame has alice as p1, firing first, and bob as p2.
func newGame(t *testing.T) game.State[Action, View] {
	t.Helper()
	st, seats, err := Rules{}.New(Opts{First: game.FirstMoverMe}, alice, []game.UserID{bob}, game.StepRNG(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if seats[alice] != p1 || seats[bob] != p2 {
		t.Fatalf("seats %v", seats)
	}
	return st
}

type move struct {
	seat   game.Seat
	action Action
}

func moves(seat game.Seat, actions ...Action) []move {
	out := make([]move, len(actions))
	for i, a := range actions {
		out[i] = move{seat: seat, action: a}
	}
	return out
}

// playFrom applies each move with the rng of its step, failing on any refusal.
func playFrom(t *testing.T, st game.State[Action, View], seed game.Seed, ms ...move) game.State[Action, View] {
	t.Helper()
	for i, m := range ms {
		st, _ = gametest.MustApply(t, Rules{}.Info(), st, m.seat, m.action, game.StepRNG(seed, game.Seq(i+1)))
	}
	return st
}

// placed is a fresh game with alice's rows fleet and bob's columns fleet, alice to fire.
func placed(t *testing.T) game.State[Action, View] {
	t.Helper()
	return playFrom(t, newGame(t), 1, append(moves(p1, rowsFleet()...), moves(p2, columnsFleet()...)...)...)
}

// shipNames are the cells of seat's ships, in fleet order.
func shipNames(st game.State[Action, View], seat game.Seat) []string {
	var names []string
	for _, ship := range st.(state).waters[seat].ships {
		for _, p := range ship.points {
			names = append(names, coords().Name(p))
		}
	}
	return names
}
