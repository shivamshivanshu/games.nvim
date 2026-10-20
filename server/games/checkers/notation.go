package checkers

import (
	"fmt"
	"slices"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

// matchMove finds the legal move whose text is exactly input, explaining why when none is.
func matchMove(input string, legal []Move) (Move, error) {
	text := strings.TrimSpace(input)
	if i := slices.IndexFunc(legal, func(m Move) bool { return m.Text() == text }); i >= 0 {
		return legal[i], nil
	}
	if continuations := movesExtending(text, legal); len(continuations) > 0 {
		return Move{}, illegal(fmt.Sprintf("illegal move %s: the jump must continue, as in %s", text, joinTexts(continuations)))
	}
	if len(legal) > 0 && legal[0].IsCapture() {
		return Move{}, illegal(fmt.Sprintf("illegal move %s: a capture is mandatory: %s", text, joinTexts(legal)))
	}
	return Move{}, illegal("illegal move: " + text)
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

func illegal(msg string) error { return game.IllegalActionError{Msg: msg} }
