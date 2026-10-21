package mastermind

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/feedback"
)

// codePattern is the catalogue's pattern for a guess, and the one the server parses it with:
// CodeLength letters of Palette in any case, with surrounding whitespace allowed.
func codePattern() string {
	return fmt.Sprintf(`^\s*([%s%s]{%d})\s*$`, Palette, strings.ToUpper(Palette), CodeLength)
}

var codeTextPattern = regexp.MustCompile(codePattern())

func parseCode(text string) (Code, error) {
	match := codeTextPattern.FindStringSubmatch(text)
	if match == nil {
		return "", game.IllegalActionError{Msg: fmt.Sprintf("a guess is %d colours from %s, got %q", CodeLength, Palette, text)}
	}
	return Code(strings.ToLower(match[1])), nil
}

// grade counts the black and white pegs of guess against secret.
func grade(guess, secret Code) Guess {
	graded := Guess{Code: guess}
	for _, mark := range feedback.Score(string(guess), string(secret)) {
		switch mark {
		case feedback.Hit:
			graded.Black++
		case feedback.Present:
			graded.White++
		}
	}
	return graded
}
