package yahtzee

import (
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

func TestBoxScore(t *testing.T) {
	cases := []struct {
		name  string
		hand  hand
		joker bool
		want  map[Box]int
	}{
		{
			name: "a small straight with a pair",
			hand: hand{3, 4, 2, 5, 3},
			want: map[Box]int{
				BoxOnes: 0, BoxTwos: 2, BoxThrees: 6, BoxFours: 4, BoxFives: 5, BoxSixes: 0,
				BoxThreeOfAKind: 0, BoxFourOfAKind: 0, BoxFullHouse: 0,
				BoxSmallStraight: 30, BoxLargeStraight: 0, BoxYahtzee: 0, BoxChance: 17,
			},
		},
		{
			name: "a large straight is also a small one",
			hand: hand{6, 2, 4, 3, 5},
			want: map[Box]int{BoxSmallStraight: 30, BoxLargeStraight: 40, BoxChance: 20},
		},
		{
			name: "a run broken in the middle is no straight",
			hand: hand{1, 2, 4, 5, 6},
			want: map[Box]int{BoxSmallStraight: 0, BoxLargeStraight: 0},
		},
		{
			name: "a full house is three of a kind but not four",
			hand: hand{2, 5, 2, 5, 2},
			want: map[Box]int{BoxThreeOfAKind: 16, BoxFourOfAKind: 0, BoxFullHouse: 25, BoxTwos: 6, BoxFives: 10},
		},
		{
			name: "four of a kind",
			hand: hand{6, 6, 1, 6, 6},
			want: map[Box]int{BoxThreeOfAKind: 25, BoxFourOfAKind: 25, BoxFullHouse: 0, BoxSixes: 24},
		},
		{
			name: "a yahtzee without the joker is no full house or straight",
			hand: hand{4, 4, 4, 4, 4},
			want: map[Box]int{
				BoxYahtzee: 50, BoxFours: 20, BoxThreeOfAKind: 20, BoxFourOfAKind: 20,
				BoxFullHouse: 0, BoxSmallStraight: 0, BoxLargeStraight: 0, BoxChance: 20,
			},
		},
		{
			name:  "a joker scores the full house and straights in full",
			hand:  hand{4, 4, 4, 4, 4},
			joker: true,
			want:  map[Box]int{BoxFullHouse: 25, BoxSmallStraight: 30, BoxLargeStraight: 40, BoxThreeOfAKind: 20},
		},
	}
	for _, tc := range cases {
		got := map[Box]int{}
		for box := range tc.want {
			got[box] = boxScore(box, tc.hand, tc.joker)
		}
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("%s %v (-want +got):\n%s", tc.name, tc.hand, diff)
		}
	}
}

// cardWith fills boxes with the given points.
func cardWith(points map[Box]int) card {
	var c card
	for box, p := range points {
		c = c.withScore(box, p, 0)
	}
	return c
}

func TestUpperBonusComesAtSixtyThree(t *testing.T) {
	threeOfEach := map[Box]int{BoxOnes: 3, BoxTwos: 6, BoxThrees: 9, BoxFours: 12, BoxFives: 15, BoxSixes: 18}
	if c := cardWith(threeOfEach); c.upperTotal() != 63 || c.upperBonus() != 35 || c.total() != 98 {
		t.Errorf("63 in the upper section: subtotal %d, bonus %d, total %d; want 63, 35, 98", c.upperTotal(), c.upperBonus(), c.total())
	}
	threeOfEach[BoxOnes] = 2
	if c := cardWith(threeOfEach); c.upperBonus() != 0 || c.total() != 62 {
		t.Errorf("62 in the upper section: bonus %d, total %d; want 0, 62", c.upperBonus(), c.total())
	}
}

func optionBoxes(options []Option) map[Box]int {
	out := map[Box]int{}
	for _, o := range options {
		out[o.Box] = o.Score
	}
	return out
}

func TestOptionsFollowTheForcedJokerRule(t *testing.T) {
	fives := hand{5, 5, 5, 5, 5}
	lower := map[Box]int{BoxThreeOfAKind: 0, BoxFourOfAKind: 0, BoxFullHouse: 0, BoxSmallStraight: 0, BoxLargeStraight: 0, BoxChance: 0}
	withYahtzee := func(points int, more map[Box]int) card {
		filled := map[Box]int{BoxYahtzee: points}
		maps.Copy(filled, more)
		return cardWith(filled)
	}
	cases := []struct {
		name      string
		card      card
		want      map[Box]int
		wantBonus int
	}{
		{
			name: "an open yahtzee box takes any open box at its plain score",
			card: cardWith(map[Box]int{BoxOnes: 1}),
			want: map[Box]int{
				BoxTwos: 0, BoxThrees: 0, BoxFours: 0, BoxFives: 25, BoxSixes: 0,
				BoxThreeOfAKind: 25, BoxFourOfAKind: 25, BoxFullHouse: 0,
				BoxSmallStraight: 0, BoxLargeStraight: 0, BoxYahtzee: 50, BoxChance: 25,
			},
		},
		{
			name:      "a joker must take its own upper box when open",
			card:      withYahtzee(50, nil),
			want:      map[Box]int{BoxFives: 25},
			wantBonus: 100,
		},
		{
			name: "with the own upper box filled, any open lower box at joker value",
			card: withYahtzee(50, map[Box]int{BoxFives: 15, BoxChance: 20}),
			want: map[Box]int{
				BoxThreeOfAKind: 25, BoxFourOfAKind: 25, BoxFullHouse: 25, BoxSmallStraight: 30, BoxLargeStraight: 40,
			},
			wantBonus: 100,
		},
		{
			name:      "with the lower section full, zero in any open upper box",
			card:      withYahtzee(50, xslices.With(xslices.With(lower, BoxFives, 15), BoxOnes, 2)),
			want:      map[Box]int{BoxTwos: 0, BoxThrees: 0, BoxFours: 0, BoxSixes: 0},
			wantBonus: 100,
		},
		{
			name: "a yahtzee box scratched with 0 still forces the joker but earns no bonus",
			card: withYahtzee(0, map[Box]int{BoxFives: 15}),
			want: map[Box]int{
				BoxThreeOfAKind: 25, BoxFourOfAKind: 25, BoxFullHouse: 25, BoxSmallStraight: 30, BoxLargeStraight: 40, BoxChance: 25,
			},
		},
	}
	for _, tc := range cases {
		if diff := cmp.Diff(tc.want, optionBoxes(tc.card.options(fives))); diff != "" {
			t.Errorf("%s (-want +got):\n%s", tc.name, diff)
		}
		if got := tc.card.bonusFor(fives); got != tc.wantBonus {
			t.Errorf("%s: bonus %d, want %d", tc.name, got, tc.wantBonus)
		}
	}
}

func TestANonYahtzeeHandIsNeverAJoker(t *testing.T) {
	c := cardWith(map[Box]int{BoxYahtzee: 50, BoxFives: 20})
	got := optionBoxes(c.options(hand{5, 5, 5, 5, 2}))
	if got[BoxFullHouse] != 0 || got[BoxFourOfAKind] != 22 || len(got) != boxCount-2 || c.bonusFor(hand{5, 5, 5, 5, 2}) != 0 {
		t.Errorf("four fives with the yahtzee box filled: options %v", got)
	}
}
