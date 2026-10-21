package poker

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
)

func parseCards(t testing.TB, text string) []cards.Card {
	t.Helper()
	var parsed []cards.Card
	for field := range strings.FieldsSeq(text) {
		c, err := cards.Parse(field)
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, c)
	}
	return parsed
}

// table deals hand handNo with the button on button; a zero stack is a seat already out.
func table(t testing.TB, handNo, button int, stacks ...Chips) state {
	t.Helper()
	s := state{n: len(stacks), subseed: 1}
	for i, stack := range stacks {
		s.users[i] = game.UserID(fmt.Sprintf("u%d", i+1))
		s.hand.chips.stack[i] = stack
		s.out[i] = stack == 0
	}
	s, _ = s.dealHand("", handNo, button)
	s, _ = s.advance("", ReasonAllChips)
	return s
}

// withCards replaces the current hand's hole cards ("p1": "As Kd") and its whole board.
func withCards(t testing.TB, s state, holes map[int]string, board string) state {
	t.Helper()
	for i, text := range holes {
		copy(s.hand.holes[i][:], parseCards(t, text))
	}
	copy(s.hand.board[:], parseCards(t, board))
	return s
}

func play(t testing.TB, s state, seat int, text string) (state, []game.Event) {
	t.Helper()
	next, events, err := s.Apply(seatName(seat), Action{Kind: ActionAct, Text: text}, nil)
	if err != nil {
		t.Fatalf("p%d %q: %v", seat+1, text, err)
	}
	return next.(state), events
}

// playAll plays "p1:call p2:check" style moves in order.
func playAll(t testing.TB, s state, moves string) (state, []game.Event) {
	t.Helper()
	var all []game.Event
	for _, step := range strings.Split(moves, ",") {
		seatText, text, _ := strings.Cut(strings.TrimSpace(step), ":")
		var seat int
		if _, err := fmt.Sscanf(seatText, "p%d", &seat); err != nil {
			t.Fatalf("bad step %q", step)
		}
		var events []game.Event
		s, events = play(t, s, seat-1, text)
		all = append(all, events...)
	}
	return s, all
}

func wantIllegal(t testing.TB, err error, want string) {
	t.Helper()
	illegal, ok := errors.AsType[game.IllegalActionError](err)
	if !ok || !strings.Contains(illegal.Msg, want) {
		t.Errorf("error %v, want an illegal action containing %q", err, want)
	}
}

func eventsOf[E game.Event](events []game.Event) []E {
	var out []E
	for _, e := range events {
		if typed, ok := e.(E); ok {
			out = append(out, typed)
		}
	}
	return out
}
