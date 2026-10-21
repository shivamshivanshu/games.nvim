package wordle

import (
	"regexp"
	"strings"
	"testing"
)

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
