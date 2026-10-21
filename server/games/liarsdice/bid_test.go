package liarsdice

import (
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game/dice"
)

func TestMinQuantity(t *testing.T) {
	cases := []struct {
		name     string
		standing *Bid
		face     dice.Face
		want     int
		ok       bool
	}{
		{"any face opens at one", nil, 4, 1, true},
		{"ones may not open", nil, Wild, 0, false},
		{"a higher face keeps the quantity", &Bid{Quantity: 3, Face: 4}, 5, 3, true},
		{"the same face needs one more", &Bid{Quantity: 3, Face: 4}, 4, 4, true},
		{"a lower face needs one more", &Bid{Quantity: 3, Face: 4}, 2, 4, true},
		{"switching to ones halves an odd quantity rounding up", &Bid{Quantity: 5, Face: 6}, Wild, 3, true},
		{"switching to ones halves an even quantity", &Bid{Quantity: 4, Face: 2}, Wild, 2, true},
		{"switching to ones from one die", &Bid{Quantity: 1, Face: 3}, Wild, 1, true},
		{"ones on ones need one more", &Bid{Quantity: 2, Face: Wild}, Wild, 3, true},
		{"switching back from ones doubles plus one", &Bid{Quantity: 2, Face: Wild}, 6, 5, true},
		{"switching back from ones to any face", &Bid{Quantity: 3, Face: Wild}, 2, 7, true},
	}
	for _, tc := range cases {
		got, ok := minQuantity(tc.standing, tc.face)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("%s: minQuantity = %d, %t; want %d, %t", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCountedTakesOnesAsWildButNotForOnes(t *testing.T) {
	cups := [][]dice.Face{{1, 1, 3, 5, 5}, {2, 5, 6}, {}}
	for face, want := range map[dice.Face]int{Wild: 2, 2: 3, 3: 3, 4: 2, 5: 5, 6: 3} {
		if got := counted(cups, face); got != want {
			t.Errorf("counted on %d = %d, want %d", face, got, want)
		}
	}
}

func TestParseBid(t *testing.T) {
	for text, want := range map[string]Bid{
		"3x5":      {By: "p1", Quantity: 3, Face: 5},
		" 12 X 2 ": {By: "p1", Quantity: 12, Face: 2},
		"0x4":      {By: "p1", Quantity: 0, Face: 4},
	} {
		if got, err := parseBid("p1", text); err != nil || got != want {
			t.Errorf("parseBid(%q) = %+v, %v; want %+v", text, got, err, want)
		}
	}
	for _, text := range []string{"", "3", "3x7", "3x0", "x5", "123x4", "3*5", "three x 5", "3x5x"} {
		if _, err := parseBid("p1", text); err == nil {
			t.Errorf("parseBid(%q) accepted", text)
		}
	}
}
