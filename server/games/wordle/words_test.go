package wordle

import (
	"regexp"
	"strings"
	"testing"
)

func isNotLowercaseLetter(r rune) bool { return r < 'a' || r > 'z' }

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
				if len(word) != WordLength || strings.ContainsFunc(word, isNotLowercaseLetter) || len(line) != len(word)+1 {
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

func TestAllowedWordsFindsBothEndsAndRejectsGaps(t *testing.T) {
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
		if got := allowed.contains(tt.word); got != tt.want {
			t.Errorf("contains(%q) = %t, want %t", tt.word, got, tt.want)
		}
	}
}

// TestEveryAnswerIsAllowed walks both sorted lists once, side by side.
func TestEveryAnswerIsAllowed(t *testing.T) {
	allowed, answers := allowedWords(), answerWords()
	var missing []string
	next := 0
	for i := range answers.len() {
		word := answers.at(i)
		for next < allowed.len() && allowed.at(next) < word {
			next++
		}
		if next == allowed.len() || allowed.at(next) != word {
			missing = append(missing, word)
		}
	}
	if len(missing) > 0 {
		t.Errorf("answers not in the allowed list: %v", missing)
	}
}

func TestCatalogueInputPatternMatchesTheGuessesParsed(t *testing.T) {
	pattern := regexp.MustCompile(Rules{}.Info().Actions[0].Input.Pattern)
	inputs := []string{
		"crane", " CRANE ", "Crane\t", "cranes", "cran", "cr4ne", "crané", "", "cr ne",
		"\u212Aiosk", "\u00a0crane", "crane\u3000", "\u2003crane",
	}
	for _, text := range inputs {
		_, err := parseGuess(text)
		shaped := err == nil || strings.HasPrefix(err.Error(), "not in word list")
		if got := pattern.MatchString(text); got != shaped {
			t.Errorf("pattern matches %q: %t, but the server parses it as a word: %t", text, got, shaped)
		}
	}
}
