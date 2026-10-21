package cards

import (
	"fmt"
	"strings"
)

// Rank is a card rank, Two (2) to Ace (14).
type Rank uint8

// Ranks from low to high.
const (
	Two Rank = iota + 2
	Three
	Four
	Five
	Six
	Seven
	Eight
	Nine
	Ten
	Jack
	Queen
	King
	Ace
)

// Suit is a card suit.
type Suit uint8

// Suits in the order of their wire letters c, d, h, s.
const (
	Clubs Suit = iota
	Diamonds
	Hearts
	Spades
)

const (
	rankLetters = "23456789TJQKA"
	suitLetters = "cdhs"
)

// FrenchSize is the cards in a French deck.
const FrenchSize = 52

// Card is one card of the French deck.
type Card struct {
	Rank Rank
	Suit Suit
}

// Invalid is the text of a value that is no card, such as the zero Card.
const Invalid = "??"

// French is the 52-card deck by rank, then suit.
func French() []Card {
	deck := make([]Card, FrenchSize)
	for i := range deck {
		deck[i] = Card{Rank: Two + Rank(i/4), Suit: Suit(i % 4)}
	}
	return deck
}

func (c Card) valid() bool { return c.Rank >= Two && c.Rank <= Ace && c.Suit <= Spades }

// String is the card's wire form: rank letter then suit letter, e.g. "As" or
// "Td"; Invalid for a value that is no card.
func (c Card) String() string {
	if !c.valid() {
		return Invalid
	}
	return string([]byte{rankLetters[c.Rank-Two], suitLetters[c.Suit]})
}

// MarshalText encodes the card in its wire form, and fails on a value that is no card.
func (c Card) MarshalText() ([]byte, error) {
	if !c.valid() {
		return nil, fmt.Errorf("card {%d %d} is no card", c.Rank, c.Suit)
	}
	return []byte(c.String()), nil
}

// JSONSchemaAlias has the wire schema describe a Card as the string it marshals to.
func (Card) JSONSchemaAlias() any { return "" }

// UnmarshalText decodes a card from its wire form.
func (c *Card) UnmarshalText(text []byte) error {
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

// Parse reads a card in its wire form, e.g. "As".
func Parse(text string) (Card, error) {
	if len(text) != 2 {
		return Card{}, fmt.Errorf("card %q: want a rank letter and a suit letter, e.g. \"As\"", text)
	}
	rank, suit := strings.IndexByte(rankLetters, text[0]), strings.IndexByte(suitLetters, text[1])
	if rank < 0 || suit < 0 {
		return Card{}, fmt.Errorf("card %q: want a rank in %s and a suit in %s", text, rankLetters, suitLetters)
	}
	return Card{Rank: Two + Rank(rank), Suit: Suit(suit)}, nil
}
