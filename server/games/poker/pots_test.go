package poker

import (
	"math/rand/v2"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUncalled(t *testing.T) {
	tests := []struct {
		name          string
		contributions []Chips
		contesting    []bool
		wantSeat      int
		wantAmount    Chips
	}{
		{"called bets", []Chips{100, 300, 300}, []bool{true, true, true}, -1, 0},
		{"excess over the second largest", []Chips{100, 500, 300}, []bool{true, true, true}, 1, 200},
		{"a dead seat's equal bet does not match", []Chips{500, 500, 300}, []bool{true, false, true}, 0, 200},
		{"a bet everyone folded to comes back whole", []Chips{20, 10, 0}, []bool{true, false, false}, 0, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seat, amount := uncalled(tt.contributions, tt.contesting)
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
			"a leaver's dead chips above the live seats' level are contested by them all", []Chips{1000, 300, 300}, []bool{false, true, true},
			[]pot{{1600, []int{1, 2}}},
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

// TestNoLiveSeatWinsALayerUncontested settles random contributions the way a
// hand does: refund the uncalled bet, then layer the rest. While two seats
// contest, every layer has two eligible; the refund is the only chips that
// leave the pots.
func TestNoLiveSeatWinsALayerUncontested(t *testing.T) {
	const cases, maxSeats, maxContribution = 10000, 6, 6
	rng := rand.New(rand.NewPCG(1, 2))
	for range cases {
		n := 2 + rng.IntN(maxSeats-1)
		contributions, contesting := make([]Chips, n), make([]bool, n)
		var total Chips
		live := 0
		for i := range n {
			contributions[i] = Chips(rng.IntN(maxContribution)) * 100
			contesting[i] = rng.IntN(3) > 0
			total += contributions[i]
			if contesting[i] {
				live++
			}
		}
		if live == 0 {
			continue
		}
		seat, back := uncalled(contributions, contesting)
		settled := append([]Chips(nil), contributions...)
		if back > 0 {
			if !contesting[seat] {
				t.Fatalf("%v %v: refund to p%d, which is not contesting", contributions, contesting, seat+1)
			}
			settled[seat] -= back
		}
		paid := back
		for _, p := range layerPots(settled, contesting) {
			paid += p.amount
			if live > 1 && len(p.eligible) < 2 {
				t.Fatalf("%v %v: pot %+v has one eligible seat of %d contesting", contributions, contesting, p, live)
			}
		}
		if paid != total {
			t.Fatalf("%v %v: refund and pots pay %d of %d", contributions, contesting, paid, total)
		}
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
