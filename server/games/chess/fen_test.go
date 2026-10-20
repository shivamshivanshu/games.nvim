package chess

import "testing"

func TestFENRoundTrip(t *testing.T) {
	fens := []string{
		StartFEN,
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
		"r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1",
		"rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8",
		"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
	}
	for _, f := range fens {
		p, err := ParseFEN(f)
		if err != nil {
			t.Fatalf("ParseFEN(%q): %v", f, err)
		}
		if got := p.FEN(); got != f {
			t.Errorf("round trip:\n got %q\nwant %q", got, f)
		}
	}
}

func TestParseFENRejects(t *testing.T) {
	cases := map[string]string{
		"7 ranks":               "rnbqkbnr/pppppppp/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"rank sums to 9":        "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR1 w KQkq - 0 1",
		"bad piece":             "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNx w KQkq - 0 1",
		"bad side":              "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR z KQkq - 0 1",
		"bad castling":          "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQx - 0 1",
		"bad en passant":        "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq e9 0 1",
		"negative halfmove":     "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - -1 1",
		"too few fields":        "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -",
		"overfull rank":         "rnbqkbnrp/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"en passant wrong rank": "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq e4 0 1",
	}
	for name, fen := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseFEN(fen); err == nil {
				t.Errorf("ParseFEN(%q) accepted", fen)
			}
		})
	}
}

func TestParseSquare(t *testing.T) {
	for s, want := range map[string]Square{"a1": 0, "h8": 63, "e4": 28} {
		got, err := ParseSquare(s)
		if err != nil || got != want {
			t.Errorf("ParseSquare(%q) = %d, %v; want %d", s, got, err, want)
		}
		if got.String() != s {
			t.Errorf("Square(%d).String() = %q; want %q", got, got.String(), s)
		}
	}
	if _, err := ParseSquare("i1"); err == nil {
		t.Error("ParseSquare(i1) accepted")
	}
}
