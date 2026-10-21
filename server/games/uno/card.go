package uno

import (
	"fmt"
	"slices"
)

// Color is a card colour by its wire letter; a wild card has none.
type Color string

// The four colours, in the order a hand is sorted by.
const (
	Red    Color = "R"
	Yellow Color = "Y"
	Green  Color = "G"
	Blue   Color = "B"
	// NoColor is a wild card's, and the colour in play before the first
	// player names one for a Wild turned up to start.
	NoColor Color = ""
)

func colors() []Color { return []Color{Red, Yellow, Green, Blue} }

// Face is what a card shows besides its colour, by its wire text.
type Face string

// The faces besides the numbers 0 to 9.
const (
	Skip         Face = "S"
	Reverse      Face = "R"
	DrawTwo      Face = "+2"
	Wild         Face = "W"
	WildDrawFour Face = "W+4"
)

// faces are every face in the order a hand is sorted by.
func faces() []Face {
	return []Face{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", Skip, Reverse, DrawTwo, Wild, WildDrawFour}
}

func (f Face) wild() bool { return f == Wild || f == WildDrawFour }

// Card is one Uno card. Its wire form is its colour letter then its face,
// e.g. "R5", "GS", "BR" (blue reverse) or "Y+2"; a wild is its face alone,
// "W" or "W+4".
type Card struct {
	Color Color
	Face  Face
}

func (c Card) String() string { return string(c.Color) + string(c.Face) }

// MarshalText encodes the card in its wire form.
func (c Card) MarshalText() ([]byte, error) { return []byte(c.String()), nil }

// JSONSchemaAlias has the wire schema describe a Card as the string it marshals to.
func (Card) JSONSchemaAlias() any { return "" }

// UnmarshalText decodes a card from its wire form.
func (c *Card) UnmarshalText(text []byte) error {
	for _, card := range orderedDeck() {
		if card.String() == string(text) {
			*c = card
			return nil
		}
	}
	return fmt.Errorf("%q is no uno card", text)
}

// cardOrder sorts a hand by colour, wilds last, then by face.
func cardOrder(a, b Card) int {
	rank := func(c Card) int {
		color := slices.Index(colors(), c.Color)
		if color < 0 {
			color = len(colors())
		}
		return color*len(faces()) + slices.Index(faces(), c.Face)
	}
	return rank(a) - rank(b)
}

const (
	coloredCopies = 2
	wildCopies    = 4
)

// orderedDeck is the 108 cards: per colour one 0, two each of 1 to 9, Skip,
// Reverse and Draw Two; then four Wild and four Wild Draw Four.
func orderedDeck() []Card {
	var deck []Card
	for _, color := range colors() {
		deck = append(deck, Card{Color: color, Face: "0"})
		for _, face := range faces()[1:] {
			if !face.wild() {
				deck = append(deck, slices.Repeat([]Card{{Color: color, Face: face}}, coloredCopies)...)
			}
		}
	}
	for _, face := range []Face{Wild, WildDrawFour} {
		deck = append(deck, slices.Repeat([]Card{{Face: face}}, wildCopies)...)
	}
	return deck
}
