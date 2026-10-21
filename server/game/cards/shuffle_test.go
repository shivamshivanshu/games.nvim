package cards

import (
	"slices"
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
