package poker

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game/cards"
)

func seven(t *testing.T, text string) [sevenSize]cards.Card {
	t.Helper()
	parsed := parseCards(t, text)
	if len(parsed) != sevenSize {
		t.Fatalf("%q has %d cards, want 7", text, len(parsed))
	}
	return [sevenSize]cards.Card(parsed)
}

func TestBestHandFindsCategoryAndTieBreakRanks(t *testing.T) {
	tests := []struct {
		name  string
		cards string
		want  HandValue
	}{
		{"high card", "As Kd 9h 7c 5s 3d 2h", HandValue{HighCard, [5]cards.Rank{cards.Ace, cards.King, cards.Nine, cards.Seven, cards.Five}}},
		{"pair with three kickers", "As Ad 9h 7c 5s 3d 2h", HandValue{Pair, [5]cards.Rank{cards.Ace, cards.Ace, cards.Nine, cards.Seven, cards.Five}}},
		{"best two of three pairs", "As Ad 9h 9c 5s 5d Kh", HandValue{TwoPair, [5]cards.Rank{cards.Ace, cards.Ace, cards.Nine, cards.Nine, cards.King}}},
		{"trips", "7s 7d 7h Ac 5s 3d 2h", HandValue{Trips, [5]cards.Rank{cards.Seven, cards.Seven, cards.Seven, cards.Ace, cards.Five}}},
		{"broadway straight", "Ts Jd Qh Kc As 2d 3h", HandValue{Straight, [5]cards.Rank{cards.Ace, cards.King, cards.Queen, cards.Jack, cards.Ten}}},
		{"wheel is five-high", "As 2d 3h 4c 5s 9d Kh", HandValue{Straight, [5]cards.Rank{cards.Five, cards.Four, cards.Three, cards.Two, cards.Ace}}},
		{"six-high over the wheel", "As 2d 3h 4c 5s 6d Kh", HandValue{Straight, [5]cards.Rank{cards.Six, cards.Five, cards.Four, cards.Three, cards.Two}}},
		{"no wrap around the ace", "Qs Kd Ah 2c 3s 8d 9h", HandValue{HighCard, [5]cards.Rank{cards.Ace, cards.King, cards.Queen, cards.Nine, cards.Eight}}},
		{"top five of six suited", "Ah Kh 9h 7h 5h 3h 2c", HandValue{Flush, [5]cards.Rank{cards.Ace, cards.King, cards.Nine, cards.Seven, cards.Five}}},
		{"flush over straight", "9h Th Jh Qh 2h Kd 3c", HandValue{Flush, [5]cards.Rank{cards.Queen, cards.Jack, cards.Ten, cards.Nine, cards.Two}}},
		{"full house from two trips", "Ks Kd Kh 9c 9s 9d 2h", HandValue{FullHouse, [5]cards.Rank{cards.King, cards.King, cards.King, cards.Nine, cards.Nine}}},
		{"full house uses the higher pair", "5s 5d 5h 9c 9s Qd Qh", HandValue{FullHouse, [5]cards.Rank{cards.Five, cards.Five, cards.Five, cards.Queen, cards.Queen}}},
		{"quads with best kicker", "8s 8d 8h 8c As Kd Kh", HandValue{Quads, [5]cards.Rank{cards.Eight, cards.Eight, cards.Eight, cards.Eight, cards.Ace}}},
		{"straight flush over a higher plain straight", "5h 6h 7h 8h 9h Ts Jd", HandValue{StraightFlush, [5]cards.Rank{cards.Nine, cards.Eight, cards.Seven, cards.Six, cards.Five}}},
		{"steel wheel", "Ah 2h 3h 4h 5h Kd Qc", HandValue{StraightFlush, [5]cards.Rank{cards.Five, cards.Four, cards.Three, cards.Two, cards.Ace}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, bestHand(seven(t, tt.cards))); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestHandValueCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"pair kicker decides", "As Ad Kh 7c 5s 3d 2h", "Ac Ah Qh 7d 5c 3s 2d", 1},
		{"second pair decides", "Ks Kd 9h 9c 2s 3d 4h", "Kc Kh 8h 8d As 3s 4d", 1},
		{"two pair kicker decides", "Ks Kd 9h 9c Qs 3d 2h", "Kc Kh 9s 9d Js 3s 2d", 1},
		{"wheel loses to six-high", "As 2d 3h 4c 5s 9d Kh", "6s 2c 3d 4h 5d 9c Kd", -1},
		{"trips rank before pair rank in a full house", "3s 3d 3h 2c 2s 9d Kh", "2d 2h 2s Ac Ad 9c Kd", 1},
		{"flush compares all five", "Ah Kh 9h 7h 5h 2c 3d", "As Ks 9s 7s 4s 2d 3c", 1},
		{"suits never break ties", "As Kd 9h 7c 5s 3d 2h", "Ah Kc 9s 7d 5c 3h 2d", 0},
		{"a board that plays ties", "2c 3d As Ks Qs Js Ts", "4h 5h As Ks Qs Js Ts", 0},
		{"sixth card does not play", "As Ad Kh Qc Js 3d 2h", "Ac Ah Kd Qh Jd 4s 3c", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := bestHand(seven(t, tt.a)), bestHand(seven(t, tt.b))
			if got := a.Compare(b); got != tt.want {
				t.Errorf("Compare = %d, want %d (%v vs %v)", got, tt.want, a, b)
			}
			if got := b.Compare(a); got != -tt.want {
				t.Errorf("reverse Compare = %d, want %d", got, -tt.want)
			}
		})
	}
}
