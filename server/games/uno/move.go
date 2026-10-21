package uno

import (
	"regexp"
	"slices"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

// playSyntax is what parsePlay accepts, and the catalogue's pattern: a
// coloured card, or a wild then the colour it names.
var playSyntax = regexp.MustCompile(`^\s*(?:([RYGBrygb])([0-9]|[SsRr]|\+2)|([Ww](?:\+4)?)\s*([RYGBrygb]))\s*$`)

// play is a card to play from the hand, and for a wild the colour it names.
type play struct {
	card  Card
	names Color
}

func parsePlay(text string) (play, error) {
	m := playSyntax.FindStringSubmatch(strings.ToUpper(text))
	if m == nil {
		return play{}, game.Illegalf(
			"%q is no card to play: want a colour letter r, y, g or b and a face 0-9, s (skip), r (reverse) or +2, e.g. r5, or w or w+4 then a colour, e.g. w g", text)
	}
	if m[3] != "" {
		return play{card: Card{Face: Face(m[3])}, names: Color(m[4])}, nil
	}
	return play{card: Card{Color: Color(m[1]), Face: Face(m[2])}}, nil
}

func parseColor(text string) (Color, error) {
	if color := Color(text); slices.Contains(colors[:], color) {
		return color, nil
	}
	return NoColor, game.Illegalf("%q is no colour: want R, Y, G or B", text)
}

func colorNames() []string {
	return xslices.Map(colors[:], func(c Color) string { return string(c) })
}
