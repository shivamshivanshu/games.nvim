package wordle

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game/feedback"
)

func TestKeyboardKeepsEachLettersBestMark(t *testing.T) {
	const (
		h = feedback.Hit
		p = feedback.Present
		m = feedback.Miss
	)
	tests := []struct {
		name    string
		guesses []Guess
		want    Keyboard
	}{
		{"no guesses", nil, Keyboard{}},
		{
			"a repeated letter takes its hit over its present",
			[]Guess{{Word: "abbey", Feedback: Feedback{p, p, h, h, m}}},
			Keyboard{"a": p, "b": h, "e": h, "y": m},
		},
		{
			"a later present upgrades a miss, a later miss never downgrades a hit",
			[]Guess{
				{Word: "stone", Feedback: Feedback{m, h, m, m, m}},
				{Word: "tests", Feedback: Feedback{m, m, p, m, m}},
			},
			Keyboard{"s": p, "t": h, "o": m, "n": m, "e": m},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, keyboardOf(tt.guesses)); diff != "" {
				t.Errorf("keyboard (-want +got):\n%s", diff)
			}
		})
	}
}
