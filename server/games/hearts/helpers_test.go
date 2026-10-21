package hearts

import (
	"slices"
	"strings"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

func users() []game.UserID { return gametest.Users(players) }

func seat(n int) game.Seat { return game.NumberedSeat(n - 1) }

func hand(t testing.TB, text string) []cards.Card {
	t.Helper()
	out := []cards.Card{}
	for field := range strings.FieldsSeq(text) {
		c, err := cards.Parse(field)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

// tricks is a hand in play at trick trickNo with p1 to lead: these hands,
// no points taken yet, hearts broken or not.
func tricks(t testing.TB, trickNo int, broken bool, hands ...string) state {
	t.Helper()
	order, _ := game.NumberSeats(users())
	s := state{
		order: order, subseed: 1, handNo: 1, scores: make([]int, players),
		hands: make([][]cards.Card, players), phase: PhasePlay,
		passes: make([][]cards.Card, players), received: make([][]cards.Card, players),
		trick: []Play{}, trickNo: trickNo, taken: make([]int, players), broken: broken,
		out: make([]bool, players),
	}
	for i, h := range hands {
		s.hands[i] = sorted(hand(t, h))
	}
	return s
}

func newGame(t testing.TB, seed game.Seed) state {
	t.Helper()
	st, _, err := Rules{}.New(Opts{}, users()[0], users()[1:], game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	return st.(opened).state
}

func playing(text string) Action { return Action{Kind: ActionPlay, Card: text} }

func passing(text string) Action { return Action{Kind: ActionPass, Cards: text} }

func apply(t testing.TB, s state, by game.Seat, a Action) (state, []game.Event) {
	t.Helper()
	return gametest.MustApply(t, Rules{}.Info(), s, by, a, nil)
}

func texts(hand []cards.Card) string {
	out := make([]string, len(hand))
	for i, c := range hand {
		out[i] = c.String()
	}
	return strings.Join(out, " ")
}

// choose is a deterministic player: it passes its three highest cards, and
// plays the queen of spades or a heart when it may, otherwise a legal card
// picked by the trick number.
func choose(v View) Action {
	if v.MustPass > 0 {
		return passing(texts(highest(v.MyHand)))
	}
	pick := v.Legal[(v.HandNo+len(v.MyHand))%len(v.Legal)]
	if at := slices.IndexFunc(v.Legal, func(c cards.Card) bool { return points(c) > 0 }); at >= 0 && len(v.Trick) > 0 {
		pick = v.Legal[at]
	}
	return playing(pick.String())
}
