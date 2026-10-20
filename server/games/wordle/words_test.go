package wordle

import (
	"strings"
	"testing"
)

func TestWordListsAreSortedFixedWidthLowercaseWords(t *testing.T) {
	for name, list := range map[string]wordList{"allowed": allowedWords(), "answers": answerWords()} {
		t.Run(name, func(t *testing.T) {
			if !strings.HasSuffix(string(list), "\n") || len(list)%lineWidth != 0 {
				t.Fatalf("length %d is not a whole number of %d-byte lines", len(list), lineWidth)
			}
			previous := ""
			for i := range list.len() {
				line := string(list[i*lineWidth : (i+1)*lineWidth])
				word := strings.TrimSuffix(line, "\n")
				if !isWord(word) || len(line) != len(word)+1 {
					t.Fatalf("line %d is %q, want %d letters a-z and a newline", i+1, line, WordLength)
				}
				if word <= previous {
					t.Fatalf("line %d: %q follows %q; want strictly increasing", i+1, word, previous)
				}
				previous = word
			}
		})
	}
}

func TestInWordListFindsBothEndsAndRejectsGaps(t *testing.T) {
	allowed := allowedWords()
	tests := []struct {
		word string
		want bool
	}{
		{allowed.at(0), true},
		{allowed.at(allowed.len() - 1), true},
		{answerWords().at(0), true},
		{"aaaaa", false},
		{"zzzzz", false},
		{"crane", true},
		{"cranx", false},
	}
	for _, tt := range tests {
		if got := inWordList(tt.word); got != tt.want {
			t.Errorf("inWordList(%q) = %t, want %t", tt.word, got, tt.want)
		}
	}
}
