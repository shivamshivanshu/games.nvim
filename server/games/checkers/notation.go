package checkers

import (
	"regexp"
	"slices"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

const (
	squarePattern = `([1-9]|[12][0-9]|3[0-2])`
	// moveSpace is the set RE2's \s matches, so trimming agrees with moveSyntax.
	moveSpace    = " \t\n\f\r"
	notationHint = "squares 1-32 joined by - for a step or x for each jump, e.g. 11-15 or 15x24x31"
)

// moveSyntax matches the move text matchMove reads: a step, or a capture listing every
// landing, each a square 1-32 without leading zeros.
var moveSyntax = regexp.MustCompile(`^\s*(` + squarePattern + stepSeparator + squarePattern +
	`|` + squarePattern + `(` + captureSeparator + squarePattern + `)+)\s*$`)

// matchMove finds the legal move whose text is exactly input, explaining why when none is.
func matchMove(input string, legal []Move) (Move, error) {
	text := strings.Trim(input, moveSpace)
	if i := slices.IndexFunc(legal, func(m Move) bool { return m.Text() == text }); i >= 0 {
		return legal[i], nil
	}
	if !moveSyntax.MatchString(text) {
		return Move{}, game.Illegalf("illegal move %q: write %s", text, notationHint)
	}
	if continuations := movesExtending(text, legal); len(continuations) > 0 {
		return Move{}, game.Illegalf("illegal move %s: the jump must continue, as in %s", text, joinTexts(continuations))
	}
	if len(legal) == 0 || !legal[0].IsCapture() {
		return Move{}, game.Illegalf("illegal move: %s", text)
	}
	if !strings.Contains(text, captureSeparator) {
		return Move{}, game.Illegalf("illegal move %s: a capture is mandatory: %s", text, joinTexts(legal))
	}
	return Move{}, game.Illegalf("illegal move %s: the captures are %s", text, joinTexts(legal))
}

func movesExtending(prefix string, legal []Move) []Move {
	if !strings.Contains(prefix, captureSeparator) {
		return nil
	}
	return slices.DeleteFunc(slices.Clone(legal), func(m Move) bool {
		return !strings.HasPrefix(m.Text(), prefix+captureSeparator)
	})
}

func joinTexts(moves []Move) string { return strings.Join(moveTexts(moves), ", ") }

func moveTexts(moves []Move) []string { return xslices.Map(moves, Move.Text) }
