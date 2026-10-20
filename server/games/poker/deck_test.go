package poker

import "testing"

func TestShuffledDeckIsAPermutationFixedBySubseedAndHand(t *testing.T) {
	d := shuffledDeck(7, 1)
	seen := map[Card]bool{}
	for _, c := range d {
		seen[c] = true
	}
	if len(seen) != deckSize {
		t.Fatalf("deck has %d distinct cards, want %d", len(seen), deckSize)
	}
	if shuffledDeck(7, 1) != d {
		t.Error("the same sub-seed and hand must deal the same deck")
	}
	if shuffledDeck(7, 2) == d || shuffledDeck(8, 1) == d {
		t.Error("another hand or sub-seed must deal another deck")
	}
}
