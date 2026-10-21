package poker

import (
	"encoding/json"
	"testing"
)

func TestAValueThatIsNoCardFailsToMarshalInsteadOfPanicking(t *testing.T) {
	for _, c := range []Card{{}, {Rank: Ace + 1, Suit: Spades}, {Rank: Ace, Suit: Spades + 1}} {
		if got := c.String(); got != invalidCard {
			t.Errorf("%#v.String() = %q, want %q", c, got, invalidCard)
		}
		if _, err := json.Marshal(c); err == nil {
			t.Errorf("%#v marshals without an error", c)
		}
	}
	if text, err := (Card{Rank: Ten, Suit: Diamonds}).MarshalText(); err != nil || string(text) != "Td" {
		t.Errorf("Td marshals to %q, %v", text, err)
	}
}
