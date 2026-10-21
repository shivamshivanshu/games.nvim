package game_test

import (
	"strconv"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func TestInfoCheckRejectsInvalidActions(t *testing.T) {
	tests := []struct {
		name    string
		actions []game.ActionSpec
		wantErr bool
	}{
		{"kind only", []game.ActionSpec{{Kind: "pass"}}, false},
		{"pattern", []game.ActionSpec{{Kind: "drop", Input: &game.ActionInput{Field: "col", Pattern: "^[1-7]$"}}}, false},
		{"values", []game.ActionSpec{{Kind: "pick", Input: &game.ActionInput{Field: "side", Values: []string{"a", "b"}}}}, false},
		{"no actions", nil, true},
		{"duplicate kind", []game.ActionSpec{{Kind: "pass"}, {Kind: "pass"}}, true},
		{"no field", []game.ActionSpec{{Kind: "drop", Input: &game.ActionInput{Pattern: "^[1-7]$"}}}, true},
		{"neither pattern nor values", []game.ActionSpec{{Kind: "drop", Input: &game.ActionInput{Field: "col"}}}, true},
		{"both pattern and values", []game.ActionSpec{{Kind: "drop", Input: &game.ActionInput{Field: "col", Pattern: "^1$", Values: []string{"1"}}}}, true},
		{"bad pattern", []game.ActionSpec{{Kind: "drop", Input: &game.ActionInput{Field: "col", Pattern: "("}}}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validInfo(func(i *game.Info) { i.Actions = tc.actions }).Check()
			if (err != nil) != tc.wantErr {
				t.Errorf("got %v, want error %t", err, tc.wantErr)
			}
		})
	}
}

func TestParseNumberedTakesOnlyTheExactValues(t *testing.T) {
	const n = 9
	for _, text := range game.NumberedValues(n) {
		if i, ok := game.ParseNumbered(text, n); !ok || strconv.Itoa(i) != text {
			t.Errorf("ParseNumbered(%q) = %d, %t", text, i, ok)
		}
	}
	for _, text := range []string{"0", "10", "+1", "01", " 1", "1 ", "", "x"} {
		if i, ok := game.ParseNumbered(text, n); ok {
			t.Errorf("ParseNumbered(%q) = %d, want none", text, i)
		}
	}
}
