package checkers

import (
	"strconv"
	"strings"
	"testing"
)

// boardOf builds a board from groups like "b:9,10 W:30": a piece code, then its squares.
func boardOf(t *testing.T, layout string) Board {
	t.Helper()
	var b Board
	for group := range strings.FieldsSeq(layout) {
		code, squares, ok := strings.Cut(group, ":")
		if !ok {
			t.Fatalf("bad layout group %q", group)
		}
		for n := range strings.SplitSeq(squares, ",") {
			sq, err := strconv.Atoi(n)
			if err != nil || sq < 1 || sq > NumSquares {
				t.Fatalf("bad square %q in %q", n, group)
			}
			b = b.with(Square(sq), Piece(code))
		}
	}
	return b
}
