package cards

import (
	"encoding/json"
	"testing"
)

func TestEveryFrenchCardReadsBackFromItsText(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range French() {
		text := c.String()
		parsed, err := Parse(text)
		if err != nil || parsed != c {
			t.Errorf("Parse(%q) = %v, %v; want %v", text, parsed, err, c)
		}
		seen[text] = true
	}
	if len(seen) != FrenchSize {
		t.Errorf("French deals %d distinct cards, want %d", len(seen), FrenchSize)
	}
}

func TestAValueThatIsNoCardFailsToMarshalInsteadOfPanicking(t *testing.T) {
	for _, c := range []Card{{}, {Rank: Ace + 1, Suit: Spades}, {Rank: Ace, Suit: Spades + 1}} {
		if got := c.String(); got != Invalid {
			t.Errorf("%#v.String() = %q, want %q", c, got, Invalid)
		}
		if _, err := json.Marshal(c); err == nil {
			t.Errorf("%#v marshals without an error", c)
		}
	}
	if text, err := (Card{Rank: Ten, Suit: Diamonds}).MarshalText(); err != nil || string(text) != "Td" {
		t.Errorf("Td marshals to %q, %v", text, err)
	}
}

func TestParseRejectsTextThatIsNoCard(t *testing.T) {
	for _, text := range []string{"", "A", "Asx", "1s", "Ax", "as", "AS"} {
		if c, err := Parse(text); err == nil {
			t.Errorf("Parse(%q) = %v, want an error", text, c)
		}
	}
}
