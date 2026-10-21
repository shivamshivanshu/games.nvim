package liarsdice

import (
	"regexp"
	"strconv"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/dice"
)

// bidPattern is what parseBid accepts, for clients to check input before sending it.
const bidPattern = `^\s*([0-9]{1,2})\s*[xX]\s*([1-6])\s*$`

var bidSyntax = regexp.MustCompile(bidPattern)

func parseBid(by game.Seat, text string) (Bid, error) {
	m := bidSyntax.FindStringSubmatch(text)
	if m == nil {
		return Bid{}, game.Illegalf("a bid is quantity x face, e.g. 3x5, got %q", text)
	}
	quantity, _ := strconv.Atoi(m[1])
	face, _ := strconv.Atoi(m[2])
	return Bid{By: by, Quantity: quantity, Face: dice.Face(face)}, nil
}

// minQuantity is the least quantity a bid on face may raise standing to, or
// false when no bid on face may follow it: the opening bid may not be on ones.
// A bid on ones counts double, so switching to ones halves the quantity,
// rounded up, and switching back doubles it plus one.
func minQuantity(standing *Bid, face dice.Face) (int, bool) {
	switch {
	case standing == nil:
		return 1, face != Wild
	case standing.Face == Wild && face == Wild:
		return standing.Quantity + 1, true
	case standing.Face == Wild:
		return 2*standing.Quantity + 1, true
	case face == Wild:
		return (standing.Quantity + 1) / 2, true
	case face > standing.Face:
		return standing.Quantity, true
	}
	return standing.Quantity + 1, true
}

// counted is how many dice in cups count toward a bid on face: those showing
// it, and the wild ones, which a bid on ones counts once.
func counted(cups [][]dice.Face, face dice.Face) int {
	n := 0
	for _, cup := range cups {
		for _, f := range cup {
			if f == face || f == Wild {
				n++
			}
		}
	}
	return n
}
