package poker

import (
	"fmt"
	"strings"
)

// Rank is a card rank, Two (2) to Ace (14).
type Rank uint8

// Ranks from low to high; an Ace also plays low in the wheel straight.
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

// Suit is a card suit. Suits never break ties.
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

// Card is one playing card.
type Card struct {
	Rank Rank
	Suit Suit
}

const invalidCard = "??"

func (c Card) valid() bool { return c.Rank >= Two && c.Rank <= Ace && c.Suit <= Spades }

// String is the card's wire form: rank letter then suit letter, e.g. "As" or
// "Td"; "??" for a value that is no card, such as the zero Card.
func (c Card) String() string {
	if !c.valid() {
		return invalidCard
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
	parsed, err := parseCard(string(text))
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

func parseCard(text string) (Card, error) {
	if len(text) != 2 {
		return Card{}, fmt.Errorf("card %q: want a rank letter and a suit letter, e.g. \"As\"", text)
	}
	rank, suit := strings.IndexByte(rankLetters, text[0]), strings.IndexByte(suitLetters, text[1])
	if rank < 0 || suit < 0 {
		return Card{}, fmt.Errorf("card %q: want a rank in %s and a suit in %s", text, rankLetters, suitLetters)
	}
	return Card{Rank: Two + Rank(rank), Suit: Suit(suit)}, nil
}
