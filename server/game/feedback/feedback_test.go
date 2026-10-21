package feedback

import (
	"slices"
	"testing"
)

func TestScore(t *testing.T) {
	const (
		h = Hit
		p = Present
		m = Miss
	)
	tests := []struct {
		name, guess, code string
		want              []Mark
	}{
		{"all hits", "crane", "crane", []Mark{h, h, h, h, h}},
		{"all misses", "fight", "crane", []Mark{m, m, m, m, m}},
		{"anagram is all present", "least", "tales", []Mark{p, p, p, p, p}},
		{"abbey against babes", "abbey", "babes", []Mark{p, p, h, h, m}},
		{"babes against abbey", "babes", "abbey", []Mark{p, p, h, h, m}},
		{"three copies against two: one hit, one present, one miss", "bobby", "abbey", []Mark{p, m, h, m, h}},
		{"two copies against one: only the first is present", "speed", "abide", []Mark{m, m, p, m, p}},
		{"a later hit takes the only copy before an earlier present", "geese", "those", []Mark{m, m, m, h, h}},
		{"hit on the first copy leaves the second a miss", "eerie", "enemy", []Mark{h, p, m, m, m}},
		{"three copies against two: the hit takes one, the first misplaced copy the other", "error", "rover", []Mark{p, p, m, p, h}},
		{"four symbols with a repeat", "rrgb", "grrb", []Mark{p, h, p, h}},
		{"symbols outside a-z", "\x00\x05\xff\x05", "\x05\x00\x05\x01", []Mark{p, p, m, p}},
		{"empty", "", "", []Mark{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Score(tt.guess, tt.code); !slices.Equal(got, tt.want) {
				t.Errorf("Score(%q, %q) = %v, want %v", tt.guess, tt.code, got, tt.want)
			}
		})
	}
}
