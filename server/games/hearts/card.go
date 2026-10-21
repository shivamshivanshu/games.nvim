package hearts

import (
	"cmp"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game/cards"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
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
var suitOrder = [...]int{cards.Clubs: 0, cards.Diamonds: 1, cards.Spades: 2, cards.Hearts: 3}

// handOrder sorts by suit, then rank from low to high.
func handOrder(a, b cards.Card) int {
	return cmp.Or(cmp.Compare(suitOrder[a.Suit], suitOrder[b.Suit]), cmp.Compare(a.Rank, b.Rank))
}

func sorted(hand []cards.Card) []cards.Card {
	out := xslices.Clone(hand)
	slices.SortFunc(out, handOrder)
	return out
}

func suitName(s cards.Suit) string {
	return [...]string{cards.Clubs: "clubs", cards.Diamonds: "diamonds", cards.Hearts: "hearts", cards.Spades: "spades"}[s]
}
