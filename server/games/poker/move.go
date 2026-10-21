package poker

import (
	"strconv"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// move is a parsed betting move; amount is the street total for bet and raise.
type move struct {
	kind   MoveKind
	amount Chips
}

const moveSyntax = "want fold, check, call, bet N, raise N or allin"

func parseMove(text string) (move, error) {
	fields := strings.Fields(strings.ToLower(text))
	if len(fields) == 0 {
		return move{}, game.Illegalf("empty move: %s", moveSyntax)
	}
	kind := MoveKind(fields[0])
	switch kind {
	case MoveFold, MoveCheck, MoveCall, MoveAllIn:
		if len(fields) != 1 {
			return move{}, game.Illegalf("%s takes no amount", kind)
		}
		return move{kind: kind}, nil
	case MoveBet, MoveRaise:
		if len(fields) != 2 {
			return move{}, game.Illegalf("%s needs one amount, e.g. %q", kind, string(kind)+" 200")
		}
		amount, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || amount <= 0 {
			return move{}, game.Illegalf("%s amount %q is not a positive whole number", kind, fields[1])
		}
		return move{kind: kind, amount: Chips(amount)}, nil
	}
	return move{}, game.Illegalf("unknown move %q: %s", fields[0], moveSyntax)
}
