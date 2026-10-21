package words

import (
	"strings"
	"testing"
)

func isNotLowercaseLetter(r rune) bool { return r < 'a' || r > 'z' }

func TestWordListsAreSortedFixedWidthLowercaseWords(t *testing.T) {
	for name, list := range map[string]List{"allowed": Allowed(), "answers": Answers()} {
		t.Run(name, func(t *testing.T) {
			if !strings.HasSuffix(string(list), "\n") || len(list)%lineWidth != 0 {
				t.Fatalf("length %d is not a whole number of %d-byte lines", len(list), lineWidth)
			}
			previous := ""
			for i := range list.Len() {
				line := string(list[i*lineWidth : (i+1)*lineWidth])
				word := strings.TrimSuffix(line, "\n")
				if len(word) != Length || strings.ContainsFunc(word, isNotLowercaseLetter) || len(line) != len(word)+1 {
					t.Fatalf("line %d is %q, want %d letters a-z and a newline", i+1, line, Length)
				}
				if word <= previous {
					t.Fatalf("line %d: %q follows %q; want strictly increasing", i+1, word, previous)
				}
				previous = word
			}
		})
	}
}

func TestAllowedWordsFindsBothEndsAndRejectsGaps(t *testing.T) {
	allowed := Allowed()
	tests := []struct {
		word string
		want bool
	}{
		{allowed.At(0), true},
		{allowed.At(allowed.Len() - 1), true},
		{Answers().At(0), true},
		{"aaaaa", false},
		{"zzzzz", false},
		{"crane", true},
		{"cranx", false},
	}
	for _, tt := range tests {
		if got := allowed.Contains(tt.word); got != tt.want {
			t.Errorf("contains(%q) = %t, want %t", tt.word, got, tt.want)
		}
	}
}

// TestEveryAnswerIsAllowed walks both sorted lists once, side by side.
func TestEveryAnswerIsAllowed(t *testing.T) {
	allowed, answers := Allowed(), Answers()
	var missing []string
	next := 0
	for i := range answers.Len() {
		word := answers.At(i)
		for next < allowed.Len() && allowed.At(next) < word {
			next++
		}
		if next == allowed.Len() || allowed.At(next) != word {
			missing = append(missing, word)
		}
	}
	if len(missing) > 0 {
		t.Errorf("answers not in the allowed list: %v", missing)
	}
}
