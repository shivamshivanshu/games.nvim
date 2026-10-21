package yahtzee

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game/dice"
)

const (
	boxCount          = 13
	upperBoxes        = 6
	upperBonusAt      = 63
	upperBonus        = 35
	fullHouseScore    = 25
	smallStraightRun  = 4
	smallStraightPays = 30
	largeStraightRun  = 5
	largeStraightPays = 40
	yahtzeeScore      = 50
	yahtzeeBonus      = 100
)

// Boxes lists every box in scorecard order: ones to sixes, then the lower section.
func Boxes() []Box {
	return []Box{
		BoxOnes, BoxTwos, BoxThrees, BoxFours, BoxFives, BoxSixes,
		BoxThreeOfAKind, BoxFourOfAKind, BoxFullHouse, BoxSmallStraight, BoxLargeStraight, BoxYahtzee, BoxChance,
	}
}

func boxIndex(box Box) (int, bool) {
	i := slices.Index(Boxes(), box)
	return i, i >= 0
}

func upperBoxOf(face dice.Face) Box { return Boxes()[face-1] }

// hand is the five dice of a turn after at least one roll.
type hand [DiceCount]dice.Face

func (h hand) counts() [dice.Six + 1]int {
	var counts [dice.Six + 1]int
	for _, face := range h {
		counts[face]++
	}
	return counts
}

func (h hand) sum() int {
	total := 0
	for _, face := range h {
		total += int(face)
	}
	return total
}

func (h hand) mostOfAKind() int {
	counts := h.counts()
	return slices.Max(counts[1:])
}

func (h hand) isYahtzee() bool { return h.mostOfAKind() == DiceCount }

func (h hand) isFullHouse() bool {
	counts := h.counts()
	return slices.Contains(counts[1:], 3) && slices.Contains(counts[1:], 2)
}

func (h hand) longestRun() int {
	counts := h.counts()
	longest, run := 0, 0
	for face := 1; face <= dice.Six; face++ {
		if counts[face] == 0 {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}
	return longest
}

// boxScore is what h scores in box. A joker scores the full house and the
// straights at their full value whatever its faces.
func boxScore(box Box, h hand, joker bool) int {
	switch box {
	case BoxThreeOfAKind:
		return sumIf(h, h.mostOfAKind() >= 3)
	case BoxFourOfAKind:
		return sumIf(h, h.mostOfAKind() >= 4)
	case BoxFullHouse:
		return valueIf(fullHouseScore, joker || h.isFullHouse())
	case BoxSmallStraight:
		return valueIf(smallStraightPays, joker || h.longestRun() >= smallStraightRun)
	case BoxLargeStraight:
		return valueIf(largeStraightPays, joker || h.longestRun() >= largeStraightRun)
	case BoxYahtzee:
		return valueIf(yahtzeeScore, h.isYahtzee())
	case BoxChance:
		return h.sum()
	}
	i, _ := boxIndex(box)
	face := i + 1
	return h.counts()[face] * face
}

func sumIf(h hand, ok bool) int { return valueIf(h.sum(), ok) }

func valueIf(value int, ok bool) int {
	if ok {
		return value
	}
	return 0
}

// card is one seat's scorecard, indexed as Boxes.
type card struct {
	filled       [boxCount]bool
	points       [boxCount]int
	yahtzeeBonus int
	out          bool
}

func (c card) isFilled(box Box) bool {
	i, _ := boxIndex(box)
	return c.filled[i]
}

func (c card) full() bool { return !slices.Contains(c.filled[:], false) }

func (c card) boxesFilled() int {
	n := 0
	for _, filled := range c.filled {
		if filled {
			n++
		}
	}
	return n
}

func (c card) upperTotal() int {
	total := 0
	for _, points := range c.points[:upperBoxes] {
		total += points
	}
	return total
}

func (c card) upperBonus() int { return valueIf(upperBonus, c.upperTotal() >= upperBonusAt) }

func (c card) total() int {
	total := c.upperBonus() + c.yahtzeeBonus
	for _, points := range c.points {
		total += points
	}
	return total
}

// isJoker is whether h is a Yahtzee rolled with the Yahtzee box already filled.
func (c card) isJoker(h hand) bool { return h.isYahtzee() && c.isFilled(BoxYahtzee) }

// bonusFor is the Yahtzee bonus h earns: one more Yahtzee with 50 already in its box.
func (c card) bonusFor(h hand) int {
	i, _ := boxIndex(BoxYahtzee)
	return valueIf(yahtzeeBonus, c.isJoker(h) && c.points[i] == yahtzeeScore)
}

// options are the boxes h may fill, under the forced joker rule: a joker must
// take its own face's upper box if open, else any open lower box, else any open upper box.
func (c card) options(h hand) []Option {
	open := slices.DeleteFunc(Boxes(), c.isFilled)
	joker := c.isJoker(h)
	if joker {
		open = jokerBoxes(open, upperBoxOf(h[0]))
	}
	options := make([]Option, len(open))
	for i, box := range open {
		options[i] = Option{Box: box, Score: boxScore(box, h, joker)}
	}
	return options
}

func jokerBoxes(open []Box, own Box) []Box {
	if slices.Contains(open, own) {
		return []Box{own}
	}
	if lower := slices.IndexFunc(open, func(b Box) bool { return !isUpper(b) }); lower >= 0 {
		return open[lower:]
	}
	return open
}

func isUpper(box Box) bool {
	i, _ := boxIndex(box)
	return i < upperBoxes
}

func (c card) withScore(box Box, points, bonus int) card {
	i, _ := boxIndex(box)
	c.filled[i], c.points[i] = true, points
	c.yahtzeeBonus += bonus
	return c
}
