package poker

import "math/rand/v2"

const deckSize = 52

// Deck is a full deck in dealing order.
type Deck [deckSize]Card

func orderedDeck() Deck {
	var d Deck
	for i := range d {
		d[i] = Card{Rank: Two + Rank(i/4), Suit: Suit(i % 4)}
	}
	return d
}

// shuffledDeck is hand handNo's deck. It depends only on the game's sub-seed,
// so Eliminate, which gets no rng, deals exactly as a replay will.
func shuffledDeck(subseed uint64, handNo int) Deck {
	d := orderedDeck()
	rng := rand.New(rand.NewPCG(subseed, uint64(handNo)))
	rng.Shuffle(len(d), func(i, j int) { d[i], d[j] = d[j], d[i] })
	return d
}
