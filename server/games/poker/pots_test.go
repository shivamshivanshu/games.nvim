package poker

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUncalled(t *testing.T) {
	tests := []struct {
		name          string
		contributions []Chips
		wantSeat      int
		wantAmount    Chips
	}{
		{"called bets", []Chips{100, 300, 300}, -1, 0},
		{"excess over the second largest", []Chips{100, 500, 300}, 1, 200},
		{"bet everyone folded to", []Chips{20, 10, 0}, 0, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seat, amount := uncalled(tt.contributions)
			if amount != tt.wantAmount || (amount > 0 && seat != tt.wantSeat) {
				t.Errorf("uncalled = (%d, %d), want (%d, %d)", seat, amount, tt.wantSeat, tt.wantAmount)
			}
		})
	}
}

func TestLayerPots(t *testing.T) {
	tests := []struct {
		name          string
		contributions []Chips
		contesting    []bool
		want          []pot
	}{
		{
			"one pot", []Chips{200, 200, 200}, []bool{true, true, true},
			[]pot{{600, []int{0, 1, 2}}},
		},
		{
			"each all-in level caps a layer", []Chips{100, 300, 500, 500}, []bool{true, true, true, true},
			[]pot{{400, []int{0, 1, 2, 3}}, {600, []int{1, 2, 3}}, {400, []int{2, 3}}},
		},
		{
			"folded chips fill the layers they reached", []Chips{200, 100, 300, 300}, []bool{false, true, true, true},
			[]pot{{400, []int{1, 2, 3}}, {500, []int{2, 3}}},
		},
		{
			"dead chips above every contested level join the top layer", []Chips{500, 300, 20}, []bool{false, true, false},
			[]pot{{820, []int{1}}},
		},
		{
			"a contested seat that put in nothing takes the dead chips", []Chips{10, 10, 0}, []bool{false, false, true},
			[]pot{{20, []int{2}}},
		},
		{"no chips, no pots", []Chips{0, 0}, []bool{true, true}, []pot{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := layerPots(tt.contributions, tt.contesting)
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(pot{})); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestSharesGiveOddChipsToTheFirstWinners(t *testing.T) {
	tests := []struct {
		amount  Chips
		winners int
		want    []Chips
	}{
		{100, 1, []Chips{100}},
		{101, 2, []Chips{51, 50}},
		{70, 3, []Chips{24, 23, 23}},
		{71, 3, []Chips{24, 24, 23}},
	}
	for _, tt := range tests {
		if diff := cmp.Diff(tt.want, shares(tt.amount, tt.winners)); diff != "" {
			t.Errorf("shares(%d, %d) (-want +got):\n%s", tt.amount, tt.winners, diff)
		}
	}
}
