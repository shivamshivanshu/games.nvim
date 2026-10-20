package checkers

import (
	"slices"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func TestSquareNumbering(t *testing.T) {
	anchors := []struct {
		sq       Square
		row, col int
	}{
		{1, 1, 2},  // b8
		{4, 1, 8},  // h8
		{5, 2, 1},  // a7
		{29, 8, 1}, // a1
		{32, 8, 7}, // g1
	}
	for _, a := range anchors {
		if a.sq.row() != a.row || a.sq.col() != a.col {
			t.Errorf("square %d at (%d,%d), want (%d,%d)", a.sq, a.sq.row(), a.sq.col(), a.row, a.col)
		}
	}
	for sq := Square(1); sq <= NumSquares; sq++ {
		if back, ok := squareAt(sq.row(), sq.col()); !ok || back != sq {
			t.Errorf("square %d does not round-trip: got %d, %t", sq, back, ok)
		}
	}
}

func TestLegalMoves(t *testing.T) {
	cases := []struct {
		name   string
		layout string
		side   game.Seat
		want   []string
	}{
		{"black opening", "", SeatBlack, []string{"9-13", "9-14", "10-14", "10-15", "11-15", "11-16", "12-16"}},
		{"white opening", "", SeatWhite, []string{"21-17", "22-17", "22-18", "23-18", "23-19", "24-19", "24-20"}},
		{"black man moves only toward white", "b:14", SeatBlack, []string{"14-17", "14-18"}},
		{"white man moves only toward black", "w:14", SeatWhite, []string{"14-9", "14-10"}},
		{"king moves both ways", "B:14", SeatBlack, []string{"14-9", "14-10", "14-17", "14-18"}},
		{"king on the edge", "W:5", SeatWhite, []string{"5-1", "5-9"}},
		{"occupied squares block steps and landings", "b:14,18,21 w:17", SeatBlack, []string{"18-22", "18-23", "21-25"}},
		{"capture is mandatory over any step", "b:9,12 w:14", SeatBlack, []string{"9x18"}},
		{"every piece's captures are offered", "b:9,10 w:14", SeatBlack, []string{"9x18", "10x17"}},
		{"man does not capture backward", "b:18 w:14", SeatBlack, []string{"18-22", "18-23"}},
		{"king captures backward", "B:18 w:14", SeatBlack, []string{"18x9"}},
		{"occupied landing blocks a capture", "b:9,18 w:14", SeatBlack, []string{"9-13", "18-22", "18-23"}},
		{"own piece cannot be jumped", "b:9,14", SeatBlack, []string{"9-13", "14-17", "14-18"}},
		{"multi-jump must be completed", "b:9 w:14,23", SeatBlack, []string{"9x18x27"}},
		{"player chooses among branches", "b:9 w:14,22,23", SeatBlack, []string{"9x18x25", "9x18x27"}},
		{"branches after a long shared prefix", "b:1 w:6,15,23,24", SeatBlack, []string{"1x10x19x26", "1x10x19x28"}},
		{"a shorter chain is legal beside a longer one", "b:9,12 w:14,16,24", SeatBlack, []string{"9x18", "12x19x28"}},
		{"crowning ends the turn mid-jump", "b:22 w:26,27", SeatBlack, []string{"22x31"}},
		{"a king jumps on through the back rank", "B:22 w:26,27", SeatBlack, []string{"22x31x24"}},
		{"white crowning ends the turn", "w:11 b:7,6", SeatWhite, []string{"11x2"}},
		{
			"a king's circuit returns to its origin and never jumps a piece twice",
			"B:10 w:14,15,22,23", SeatBlack,
			[]string{"10x17x26x19x10", "10x19x26x17x10"},
		},
		{"no pieces, no moves", "w:14", SeatBlack, nil},
		{"blocked man has no move", "b:12 w:16,19", SeatBlack, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := StartBoard()
			if c.layout != "" {
				b = boardOf(t, c.layout)
			}
			if got := moveTexts(b.LegalMoves(c.side)); !slices.Equal(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestPlay(t *testing.T) {
	cases := []struct {
		name        string
		layout      string
		side        game.Seat
		move        string
		want        string
		wantCrowned bool
	}{
		{"step", "b:9", SeatBlack, "9-13", "b:13", false},
		{"jumped pieces are removed once the move ends", "b:9 w:14,23 W:32", SeatBlack, "9x18x27", "b:27 W:32", false},
		{"black man crowns on row 8", "b:25", SeatBlack, "25-29", "B:29", true},
		{"white man crowns on row 1 by capture", "w:11 b:7,6", SeatWhite, "11x2", "W:2 b:6", true},
		{"a king does not crown again", "B:25", SeatBlack, "25-29", "B:29", false},
		{"circuit lands the king on its origin", "B:10 w:14,15,22,23", SeatBlack, "10x17x26x19x10", "B:10", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := boardOf(t, c.layout)
			legal := b.LegalMoves(c.side)
			i := slices.IndexFunc(legal, func(m Move) bool { return m.Text() == c.move })
			if i < 0 {
				t.Fatalf("%s not legal in %v", c.move, moveTexts(legal))
			}
			got, crowned := b.Play(legal[i])
			if want := boardOf(t, c.want); got != want || crowned != c.wantCrowned {
				t.Errorf("got %v crowned=%t, want %v crowned=%t", got, crowned, want, c.wantCrowned)
			}
		})
	}
}
