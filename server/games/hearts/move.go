package hearts

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
)

const cardSyntax = `[2-9TtJjQqKkAa][CcDdHhSs]`

// playSyntax and passSyntax are what parsePlay and parsePass accept, and the
// catalogue's patterns.
var (
	playSyntax = regexp.MustCompile(`^\s*(` + cardSyntax + `)\s*$`)
	passSyntax = regexp.MustCompile(`^\s*(` + cardSyntax + `)\s+(` + cardSyntax + `)\s+(` + cardSyntax + `)\s*$`)
)

// cardOf reads one card as the patterns match it: a rank, then a suit, any case.
func cardOf(text string) cards.Card {
	c, err := cards.Parse(strings.ToUpper(text[:1]) + strings.ToLower(text[1:]))
	if err != nil {
		panic(fmt.Sprintf("card pattern matched %q, which is no card: %v", text, err))
	}
	return c
}

func parsePlay(text string) (cards.Card, error) {
	m := playSyntax.FindStringSubmatch(text)
	if m == nil {
		return cards.Card{}, game.Illegalf("%q is no card: want a rank 2-9, t, j, q, k or a, then a suit c, d, h or s, e.g. qs", text)
	}
	return cardOf(m[1]), nil
}

func parsePass(text string) ([]cards.Card, error) {
	m := passSyntax.FindStringSubmatch(text)
	if m == nil {
		return nil, game.Illegalf("%q is not three cards: want three cards apart, e.g. 2c qs ah", text)
	}
	pass := []cards.Card{cardOf(m[1]), cardOf(m[2]), cardOf(m[3])}
	if pass[0] == pass[1] || pass[0] == pass[2] || pass[1] == pass[2] {
		return nil, game.Illegalf("%q names a card twice: pass three different cards", text)
	}
	return pass, nil
}
