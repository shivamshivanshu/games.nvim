package poker

import (
	"cmp"
	"fmt"
	"slices"
)

// Category is a poker hand category, ordered from weakest to strongest.
type Category uint8

// Categories from weakest to strongest.
const (
	HighCard Category = iota
	Pair
	TwoPair
	Trips
	Straight
	Flush
	FullHouse
	Quads
	StraightFlush
)

// String is the category's wire name, e.g. "full_house".
func (c Category) String() string {
	return [...]string{
		"high_card", "pair", "two_pair", "trips", "straight", "flush", "full_house", "quads", "straight_flush",
	}[c]
}

// MarshalText encodes the category as its wire name.
func (c Category) MarshalText() ([]byte, error) { return []byte(c.String()), nil }

// UnmarshalText decodes a category from its wire name.
func (c *Category) UnmarshalText(text []byte) error {
	for k := HighCard; k <= StraightFlush; k++ {
		if k.String() == string(text) {
			*c = k
			return nil
		}
	}
	return fmt.Errorf("unknown hand category %q", text)
}

const (
	handSize  = 5
	sevenSize = 7
)

// HandValue scores a five-card hand: the category, then the ranks that break
// ties within it, most significant first.
type HandValue struct {
	Category Category
	Ranks    [handSize]Rank
}

// Compare orders hand values: negative when v loses to o, 0 on a tie.
func (v HandValue) Compare(o HandValue) int {
	if c := cmp.Compare(v.Category, o.Category); c != 0 {
		return c
	}
	return slices.Compare(v.Ranks[:], o.Ranks[:])
}

// bestHand is the best five-card value among all 21 choices from seven cards.
func bestHand(cards [sevenSize]Card) HandValue {
	var best HandValue
	for skipA := range sevenSize {
		for skipB := skipA + 1; skipB < sevenSize; skipB++ {
			var five [handSize]Card
			k := 0
			for i, c := range cards {
				if i != skipA && i != skipB {
					five[k] = c
					k++
				}
			}
			if v := evaluate5(five); v.Compare(best) > 0 {
				best = v
			}
		}
	}
	return best
}

func evaluate5(hand [handSize]Card) HandValue {
	var count [Ace + 1]int
	for _, c := range hand {
		count[c.Rank]++
	}
	distinct := make([]Rank, 0, handSize)
	for r := Ace; r >= Two; r-- {
		if count[r] > 0 {
			distinct = append(distinct, r)
		}
	}
	slices.SortStableFunc(distinct, func(a, b Rank) int { return cmp.Compare(count[b], count[a]) })
	var grouped [handSize]Rank
	k := 0
	for _, r := range distinct {
		for range count[r] {
			grouped[k] = r
			k++
		}
	}
	high, straight := straightHigh(distinct)
	flush := isFlush(hand)
	largest, second := count[distinct[0]], count[distinct[1]]
	switch {
	case straight && flush:
		return HandValue{StraightFlush, straightRanks(high)}
	case largest == 4:
		return HandValue{Quads, grouped}
	case largest == 3 && second == 2:
		return HandValue{FullHouse, grouped}
	case flush:
		return HandValue{Flush, grouped}
	case straight:
		return HandValue{Straight, straightRanks(high)}
	case largest == 3:
		return HandValue{Trips, grouped}
	case largest == 2 && second == 2:
		return HandValue{TwoPair, grouped}
	case largest == 2:
		return HandValue{Pair, grouped}
	}
	return HandValue{HighCard, grouped}
}

func isFlush(hand [handSize]Card) bool {
	for _, c := range hand[1:] {
		if c.Suit != hand[0].Suit {
			return false
		}
	}
	return true
}

// straightHigh reads five distinct ranks, highest first. The wheel A-5-4-3-2
// is five-high, and straights do not wrap around the ace.
func straightHigh(distinct []Rank) (Rank, bool) {
	if len(distinct) != handSize {
		return 0, false
	}
	if distinct[0]-distinct[handSize-1] == handSize-1 {
		return distinct[0], true
	}
	if slices.Equal(distinct, []Rank{Ace, Five, Four, Three, Two}) {
		return Five, true
	}
	return 0, false
}

func straightRanks(high Rank) [handSize]Rank {
	var ranks [handSize]Rank
	for i := range ranks {
		ranks[i] = high - Rank(i)
	}
	if high == Five {
		ranks[handSize-1] = Ace
	}
	return ranks
}
