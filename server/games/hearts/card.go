package hearts

import (
	"cmp"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game/cards"
)

var (
	twoOfClubs    = cards.Card{Rank: cards.Two, Suit: cards.Clubs}
	queenOfSpades = cards.Card{Rank: cards.Queen, Suit: cards.Spades}
)

// points is what taking c costs: 1 for a heart, 13 for the queen of spades.
func points(c cards.Card) int {
	switch {
	case c.Suit == cards.Hearts:
		return 1
	case c == queenOfSpades:
		return queenPoints
	}
	return 0
}

// suitOrder sorts a hand clubs, diamonds, spades, hearts, so colours alternate.
func suitOrder(s cards.Suit) int {
	return slices.Index([]cards.Suit{cards.Clubs, cards.Diamonds, cards.Spades, cards.Hearts}, s)
}

// handOrder sorts by suit, then rank from low to high.
func handOrder(a, b cards.Card) int {
	return cmp.Or(cmp.Compare(suitOrder(a.Suit), suitOrder(b.Suit)), cmp.Compare(a.Rank, b.Rank))
}

func sorted(hand []cards.Card) []cards.Card {
	out := append([]cards.Card{}, hand...)
	slices.SortFunc(out, handOrder)
	return out
}

func suitName(s cards.Suit) string {
	return [...]string{cards.Clubs: "clubs", cards.Diamonds: "diamonds", cards.Hearts: "hearts", cards.Spades: "spades"}[s]
}
