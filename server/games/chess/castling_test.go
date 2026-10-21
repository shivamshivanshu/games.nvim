package chess

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func backRankOf(t *testing.T, letters string) BackRank {
	t.Helper()
	var rank BackRank
	for i := range len(letters) {
		piece, ok := pieceFromLetter(letters[i])
		if !ok || len(letters) != fileCount {
			t.Fatalf("back rank %q", letters)
		}
		rank[i] = piece.Kind
	}
	return rank
}

// chess960At is fen in the game that started from rank, its castling field
// read as the rights to castle with the rooks either side of the king there.
func chess960At(t *testing.T, rank, fen string) Position {
	t.Helper()
	start, err := Chess960Position(backRankOf(t, rank))
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(fen)
	rights, err := parseCastling(fields[2])
	if err != nil {
		t.Fatal(err)
	}
	fields[2] = noneField
	p := mustFEN(t, strings.Join(fields, " "))
	p.Castling, p.layout = rights, start.layout
	return p
}

func castleUCIs(p Position) []string {
	var ucis []string
	for _, m := range p.LegalMoves() {
		if _, ok := p.castlingByKingMove(p.Board[m.From], m); ok {
			ucis = append(ucis, m.UCI())
		}
	}
	return ucis
}

func TestChess960StartPosition(t *testing.T) {
	p, err := Chess960Position(backRankOf(t, "RNBQKBNR"))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.FEN(); got != StartFEN {
		t.Errorf("FEN %s, want %s", got, StartFEN)
	}
	if got := castleUCIs(p); len(got) != 0 {
		t.Errorf("castles %v at the start", got)
	}
	for _, rank := range []string{"RNBQKBNN", "KRNBBQNR", "RKRNNQBK", "RNBQRBNK", "RNBKKBNR"} {
		if _, err := Chess960Position(backRankOf(t, rank)); err == nil {
			t.Errorf("%s: want an error, the king not between two rooks", rank)
		}
	}
}

func TestChess960Castling(t *testing.T) {
	tests := []struct {
		name, rank, fen string
		wantCastles     []string
		play, wantFEN   string
	}{
		{
			"the standard array castles onto the rook", "RNBQKBNR",
			"r3k2r/pppppppp/8/8/8/8/PPPPPPPP/R3K2R w KQkq - 0 1", []string{"e1h1", "e1a1"},
			"O-O", "r3k2r/pppppppp/8/8/8/8/PPPPPPPP/R4RK1 b kq - 1 1",
		},
		{
			"a king on g1 stays, its rook goes to f1", "RNBQNBKR",
			"rnbqnbkr/pppppppp/8/8/8/8/PPPPPPPP/RNBQN1KR w KQkq - 0 1", []string{"g1h1"},
			"g1h1", "rnbqnbkr/pppppppp/8/8/8/8/PPPPPPPP/RNBQNRK1 b kq - 1 1",
		},
		{
			"a king on b1 goes to c1, its rook to d1", "RKNBBQNR",
			"rknbbqnr/pppppppp/8/8/8/8/PPPPPPPP/RK2BQNR w KQkq - 0 1", []string{"b1a1"},
			"O-O-O", "rknbbqnr/pppppppp/8/8/8/8/PPPPPPPP/2KRBQNR b kq - 1 1",
		},
		{
			"king and rook swap squares from the start", "RNBQBKRN",
			"rnbqbkrn/pppppppp/8/8/8/8/PPPPPPPP/RNBQBKRN w KQkq - 0 1", []string{"f1g1"},
			"0-0", "rnbqbkrn/pppppppp/8/8/8/8/PPPPPPPP/RNBQBRKN b kq - 1 1",
		},
		{
			"a plain king step to g1 is not the castle", "RNBQBKNR",
			"rnbqbknr/pppppppp/8/8/8/8/PPPPPPPP/RNBQBK1R w KQkq - 0 1", []string{"f1h1"},
			"Kg1", "rnbqbknr/pppppppp/8/8/8/8/PPPPPPPP/RNBQB1KR b kq - 1 1",
		},
		{
			"a piece on a square the king crosses blocks it", "RKNBBQNR",
			"rknbbqnr/pppppppp/8/8/8/8/PPPPPPPP/RK4NR w KQkq - 0 1", []string{"b1a1"},
			"", "",
		},
		{
			"the king may not cross an attacked square", "RKNBBQNR",
			"1k2r3/8/8/8/8/8/8/RK5R w KQ - 0 1", []string{"b1a1"},
			"", "",
		},
		{
			"the castling rook may not be what shields the king", "NRNBBKRQ",
			"4k3/8/8/8/8/8/8/rR3K2 w Q - 0 1", nil,
			"", "",
		},
		{
			"a plain king step onto the square its rook left", "RNBQBKRN",
			"rnbqbkrn/pppppppp/8/8/8/8/PPPPPPPP/RNBQBK1N w Qkq - 0 1", nil,
			"Kg1", "rnbqbkrn/pppppppp/8/8/8/8/PPPPPPPP/RNBQB1KN b kq - 1 1",
		},
		{
			"no castling out of check", "RNBQKBNR",
			"4r1k1/8/8/8/8/8/8/R3K2R w KQ - 0 1", nil,
			"", "",
		},
		{
			"moving a castling rook loses its right", "RKNBBQNR",
			"rknbbqnr/pppppppp/8/8/8/8/PPPPPPP1/RK2BQNR w KQkq - 0 1", []string{"b1a1"},
			"Rh4", "rknbbqnr/pppppppp/8/8/7R/8/PPPPPPP1/RK2BQN1 b Qkq - 1 1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := chess960At(t, tc.rank, tc.fen)
			got := castleUCIs(p)
			slices.Sort(got)
			want := slices.Sorted(slices.Values(tc.wantCastles))
			if !slices.Equal(got, want) {
				t.Errorf("castles %v, want %v", got, want)
			}
			if tc.play == "" {
				return
			}
			if got := p.Play(mustParse(t, p, tc.play)).FEN(); got != tc.wantFEN {
				t.Errorf("after %s: %s, want %s", tc.play, got, tc.wantFEN)
			}
		})
	}
}

func TestChess960CastleNotation(t *testing.T) {
	p := chess960At(t, "RNBQBKNR", "rnbqbknr/pppppppp/8/8/8/8/PPPPPPPP/RNBQBK1R w KQkq - 0 1")
	castle := mustParse(t, p, "O-O")
	if castle.UCI() != "f1h1" || p.SAN(castle) != "O-O" {
		t.Errorf("O-O is %s, written %s", castle.UCI(), p.SAN(castle))
	}
	if step := mustParse(t, p, "f1g1"); p.SAN(step) != "Kg1" {
		t.Errorf("f1g1 written %s", p.SAN(step))
	}

	standardArray := chess960At(t, "RNBQKBNR", "r3k2r/pppppppp/8/8/8/8/PPPPPPPP/R3K2R w KQkq - 0 1")
	if m, err := standardArray.ParseMove("e1g1"); err == nil {
		t.Errorf("e1g1 read as %s; a Chess960 castle is the king onto its rook", m.UCI())
	}
	if got := mustParse(t, standardArray, "e1a1"); standardArray.SAN(got) != "O-O-O" {
		t.Errorf("e1a1 written %s", standardArray.SAN(got))
	}
}

// TestChess960Perft counts against python-chess 1.11.2 in Chess960 mode.
func TestChess960Perft(t *testing.T) {
	cases := []struct {
		name, rank, fen string
		nodes           []uint64
	}{
		{"position 0", "BBQNNRKR", "bbqnnrkr/pppppppp/8/8/8/8/PPPPPPPP/BBQNNRKR w KQkq - 0 1", []uint64{20, 400, 9006, 201143}},
		{"rooks b and g", "QRBBKNRN", "1r2k1r1/pppppppp/8/8/8/8/PPPPPPPP/1R2K1R1 w KQkq - 0 1", []uint64{25, 625, 15131, 366277}},
		{"king on c", "RNKBBQNR", "r1k4r/8/8/8/8/8/8/R1K4R w KQkq - 0 1", []uint64{26, 570, 13776, 313736}},
		{"king and queenside rook close", "NRKBBQRN", "1rk3r1/1pp2pp1/8/8/8/8/1PP2PP1/1RK3R1 w KQkq - 0 1", []uint64{17, 282, 5236, 95607}},
		{"rook shield", "NRNBBKRQ", "4k3/8/8/8/8/8/8/rR3K2 w Q - 0 1", []uint64{9, 102, 1501, 22885}},
	}
	for _, c := range cases {
		p := chess960At(t, c.rank, c.fen)
		for i, want := range c.nodes {
			t.Run(fmt.Sprintf("%s/depth%d", c.name, i+1), func(t *testing.T) {
				if got := perft(p, i+1); got != want {
					t.Errorf("perft = %d, want %d", got, want)
				}
			})
		}
	}
}
