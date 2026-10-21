package cards

import (
	"slices"
	"strings"
	"testing"
)

func TestShuffledIsAPermutationFixedBySubseedAndN(t *testing.T) {
	ordered := French()
	deck := Shuffled(ordered, 7, 1)
	if !slices.Equal(ordered, French()) {
		t.Fatal("Shuffled changed the deck it was given")
	}
	sorted := slices.SortedFunc(slices.Values(deck), func(a, b Card) int {
		return slices.Index(ordered, a) - slices.Index(ordered, b)
	})
	if !slices.Equal(sorted, ordered) {
		t.Fatalf("shuffled deck %v is not a permutation of the deck", deck)
	}
	if !slices.Equal(Shuffled(ordered, 7, 1), deck) {
		t.Error("the same sub-seed and n must shuffle the same deck")
	}
	if slices.Equal(Shuffled(ordered, 7, 2), deck) || slices.Equal(Shuffled(ordered, 8, 1), deck) {
		t.Error("another n or sub-seed must shuffle another deck")
	}
}

// TestShuffledDealsTheSameDeckAsEver pins one shuffle, so that a change to
// Shuffled, which would change every deal and break the replay of every
// archived card game, cannot pass unnoticed.
func TestShuffledDealsTheSameDeckAsEver(t *testing.T) {
	const want = "Kc Jd 5h Ad Qs Ac 2s Ts 9d 8h 9c 3d Qh Ah Ks 9s 3h Jh Kh 8d 6c 8s Kd Qd 6s Jc " +
		"4d 7d 5c 9h 6h Js 2d 5s As Th 8c 2c Td 7h 6d 3s 3c 4c 7c 4h 4s 7s 5d 2h Qc Tc"
	got := make([]string, 0, FrenchSize)
	for _, c := range Shuffled(French(), 7, 1) {
		got = append(got, c.String())
	}
	if strings.Join(got, " ") != want {
		t.Errorf("Shuffled(French(), 7, 1) = %s\nwant %s", strings.Join(got, " "), want)
	}
}
