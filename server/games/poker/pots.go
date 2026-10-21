package poker

import (
	"slices"
)

// pot is one layer of the chips in a hand and the seats that may win it.
type pot struct {
	amount   Chips
	eligible []int
}

// uncalled is the part of the largest contesting contribution above the
// second largest contesting one: no live seat matched it, so it goes back to
// its owner. Chips of a seat that folded or left never count as matching, or
// the owner would win them uncontested.
func uncalled(contributions []Chips, contesting []bool) (seat int, amount Chips) {
	seat = -1
	var top, second Chips
	for i, c := range contributions {
		switch {
		case !contesting[i]:
		case c > top:
			seat, top, second = i, c, top
		case c > second:
			second = c
		}
	}
	return seat, top - second
}

// layerPots splits contributions into a main pot and side pots, one layer per
// distinct contribution of a seat still contesting the pot. Folded seats'
// chips fill the layers they reached but win nothing; chips above every
// contesting seat's level join the top layer. Empty layers are dropped.
func layerPots(contributions []Chips, contesting []bool) []pot {
	var levels []Chips
	for i, c := range contributions {
		if contesting[i] {
			levels = append(levels, c)
		}
	}
	slices.Sort(levels)
	levels = slices.Compact(levels)
	pots := make([]pot, 0, len(levels))
	var floor Chips
	for _, level := range levels {
		layer := pot{}
		for i, c := range contributions {
			layer.amount += min(c, level) - min(c, floor)
			if contesting[i] && c >= level {
				layer.eligible = append(layer.eligible, i)
			}
		}
		pots = append(pots, layer)
		floor = level
	}
	if len(pots) > 0 {
		for _, c := range contributions {
			pots[len(pots)-1].amount += max(0, c-floor)
		}
	}
	return slices.DeleteFunc(pots, func(p pot) bool { return p.amount == 0 })
}

// shares splits amount equally among winners, in odd-chip order: the odd
// chips go one at a time to the first winners.
func shares(amount Chips, winners int) []Chips {
	each, odd := amount/Chips(winners), amount%Chips(winners)
	out := make([]Chips, winners)
	for i := range out {
		out[i] = each
		if Chips(i) < odd {
			out[i]++
		}
	}
	return out
}
