package game_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func TestPairSeatsAndAssigns(t *testing.T) {
	pair := game.Pair{"x", "o"}
	if pair.Other("x") != "o" || pair.Other("o") != "x" {
		t.Errorf("Other: x faces %q, o faces %q", pair.Other("x"), pair.Other("o"))
	}
	if want := []game.Seat{"x", "o"}; !cmp.Equal(pair.Seats(), want) {
		t.Errorf("Seats() = %v, want %v", pair.Seats(), want)
	}
	challengerFirst := map[game.UserID]game.Seat{"alice": "x", "bob": "o"}
	opponentFirst := map[game.UserID]game.Seat{"alice": "o", "bob": "x"}
	tests := []struct {
		first game.FirstMover
		seed  game.Seed
		want  map[game.UserID]game.Seat
	}{
		{game.FirstMoverMe, 1, challengerFirst},
		{game.FirstMoverOpponent, 1, opponentFirst},
		{game.FirstMoverRandom, 1, opponentFirst},
		{game.FirstMoverRandom, 6, challengerFirst},
	}
	for _, tc := range tests {
		if got := pair.Assign(tc.first, "alice", []game.UserID{"bob"}, game.StepRNG(tc.seed, 0)); !cmp.Equal(got, tc.want) {
			t.Errorf("Assign(%s, seed %d) = %v, want %v", tc.first, tc.seed, got, tc.want)
		}
	}
}

func TestSideOptionChoosesTheFirstMover(t *testing.T) {
	spec := game.SideOption("white", "black")
	if want := game.OptionValues("random", "white", "black"); spec.Name != "color" || !cmp.Equal(spec.Values, want) || spec.Default != "random" {
		t.Errorf("SideOption = %+v", spec)
	}
	for choice, want := range map[game.SideChoice]game.FirstMover{
		"white":  game.FirstMoverMe,
		"black":  game.FirstMoverOpponent,
		"random": game.FirstMoverRandom,
		"":       game.FirstMoverRandom,
	} {
		if got := game.SideFirstMover(choice, "white", "black"); got != want {
			t.Errorf("SideFirstMover(%q) = %q, want %q", choice, got, want)
		}
	}
}
