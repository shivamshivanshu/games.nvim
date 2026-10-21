package hangman

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/words"
)

// guessPattern is the catalogue's pattern for a guess, and the one the server parses it with:
// one ASCII letter or a whole word of them, with surrounding whitespace allowed.
func guessPattern() string {
	return fmt.Sprintf(`^\s*([A-Za-z]|[A-Za-z]{%d})\s*$`, words.Length)
}

var guessTextPattern = regexp.MustCompile(guessPattern())

// parseGuess returns the lowercase letter or word text guesses; a word must be in the word list.
func parseGuess(text string) (string, error) {
	match := guessTextPattern.FindStringSubmatch(text)
	if match == nil {
		return "", game.Illegalf("a guess is one letter a-z or a %d-letter word, got %q", words.Length, text)
	}
	guess := strings.ToLower(match[1])
	if len(guess) == words.Length && !words.Allowed().Contains(guess) {
		return "", game.Illegalf("not in word list: %s", guess)
	}
	return guess, nil
}
