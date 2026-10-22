package uno

import (
	"strings"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

func seat(n int) game.Seat { return game.NumberedSeat(n - 1) }

func card(t testing.TB, text string) Card {
	t.Helper()
	var c Card
	if err := c.UnmarshalText([]byte(text)); err != nil {
		t.Fatal(err)
	}
	return c
}

func hand(t testing.TB, text string) []Card {
	t.Helper()
	out := []Card{}
	for field := range strings.FieldsSeq(text) {
		out = append(out, card(t, field))
	}
	return out
}

// table is a hand in play: p1 to act clockwise, these hands, top alone on
// the discard pile and in play by its own colour, and pile the draw pile, top first.
func table(t testing.TB, top, pile string, hands ...string) state {
	t.Helper()
	order, _ := game.NumberSeats(gametest.Users(len(hands)))
	s := state{
		order:   order,
		hands:   make([][]Card, len(hands)),
		pile:    hand(t, pile),
		discard: []Card{card(t, top)},
		color:   card(t, top).Color,
		step:    clockwiseStep,
		phase:   PhasePlay,
		out:     make([]bool, len(hands)),
		subseed: 1,
	}
	for i, h := range hands {
		s.hands[i] = hand(t, h)
	}
	return s
}

func playing(text string) Action { return Action{Kind: ActionPlay, Card: text} }

func kind(k game.ActionKind) Action { return Action{Kind: k} }

func apply(t testing.TB, s state, by game.Seat, a Action) (state, []game.Event) {
	t.Helper()
	return gametest.MustApply(t, Rules{}.Info(), s, by, a, nil)
}

func newGame(t testing.TB, n int, seed game.Seed) state {
	t.Helper()
	return gametest.NewState[opened](t, Rules{}, Opts{}, gametest.Users(n), seed).state
}

// choose is a deterministic player: it plays what matches, coloured cards
// before wilds but now and then a Wild Draw Four first, bluff or not, naming
// the colour it holds most of; it draws when nothing matches, plays what it
// drew, and challenges every other Wild Draw Four.
func choose(v View) Action {
	switch v.Phase {
	case PhaseColor:
		return Action{Kind: ActionColor, Color: string(mostHeld(v.MyHand))}
	case PhaseChallenge:
		if len(v.MyHand)%2 == 0 {
			return kind(ActionChallenge)
		}
		return kind(ActionAccept)
	}
	if len(v.Playable) == 0 && v.Phase == PhaseDrawn {
		return kind(ActionPass)
	}
	if len(v.Playable) == 0 {
		return kind(ActionDraw)
	}
	pick := v.Playable[0]
	if last := v.Playable[len(v.Playable)-1]; last.Face == WildDrawFour && len(v.MyHand)%3 == 0 {
		pick = last
	}
	if pick.Face.wild() {
		return playing(pick.String() + " " + string(mostHeld(v.MyHand)))
	}
	return playing(pick.String())
}

func mostHeld(h []Card) Color {
	best, most := Red, -1
	for _, color := range colors {
		n := 0
		for _, c := range h {
			if c.Color == color {
				n++
			}
		}
		if n > most {
			best, most = color, n
		}
	}
	return best
}
