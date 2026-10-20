package wordle

import "testing"

func TestScore(t *testing.T) {
	const (
		h = MarkHit
		p = MarkPresent
		m = MarkMiss
	)
	tests := []struct {
		name, guess, answer string
		want                Feedback
	}{
		{"all hits", "crane", "crane", Feedback{h, h, h, h, h}},
		{"all misses", "fight", "crane", Feedback{m, m, m, m, m}},
		{"anagram is all present", "least", "tales", Feedback{p, p, p, p, p}},
		{"abbey against babes", "abbey", "babes", Feedback{p, p, h, h, m}},
		{"babes against abbey", "babes", "abbey", Feedback{p, p, h, h, m}},
		{"three copies against two: one hit, one present, one miss", "bobby", "abbey", Feedback{p, m, h, m, h}},
		{"two copies against one: only the first is present", "speed", "abide", Feedback{m, m, p, m, p}},
		{"a later hit takes the only copy before an earlier present", "geese", "those", Feedback{m, m, m, h, h}},
		{"hit on the first copy leaves the second a miss", "eerie", "enemy", Feedback{h, p, m, m, m}},
		{"three copies against two: the hit takes one, the first misplaced copy the other", "error", "rover", Feedback{p, p, m, p, h}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := score(tt.guess, tt.answer); got != tt.want {
				t.Errorf("score(%q, %q) = %v, want %v", tt.guess, tt.answer, got, tt.want)
			}
		})
	}
}
