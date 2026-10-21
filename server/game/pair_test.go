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
	for challengerFirst, want := range map[bool]map[game.UserID]game.Seat{
		true:  {"alice": "x", "bob": "o"},
		false: {"alice": "o", "bob": "x"},
	} {
		if got := pair.Assign("alice", "bob", challengerFirst); !cmp.Equal(got, want) {
			t.Errorf("Assign(challengerFirst %t) = %v, want %v", challengerFirst, got, want)
		}
	}
}

func TestSideOptionChoosesTheFirstMover(t *testing.T) {
	spec := game.SideOption("white", "black")
	if want := game.OptionValues("random", "white", "black"); spec.Name != "color" || !cmp.Equal(spec.Values, want) || spec.Default != "random" {
		t.Errorf("SideOption = %+v", spec)
	}
	for choice, want := range map[string]game.FirstMover{
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
