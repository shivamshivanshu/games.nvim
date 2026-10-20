package chess

import (
	"fmt"
	"testing"
)

const shortModeNodeLimit = 1_000_000

func perft(p Position, depth int) uint64 {
	if depth == 0 {
		return 1
	}
	moves := p.LegalMoves()
	if depth == 1 {
		return uint64(len(moves))
	}
	var nodes uint64
	for _, m := range moves {
		nodes += perft(p.Play(m), depth-1)
	}
	return nodes
}

func TestPerft(t *testing.T) {
	cases := []struct {
		name  string
		fen   string
		nodes []uint64
	}{
		{"start", StartFEN, []uint64{20, 400, 8902, 197281, 4865609}},
		{"kiwipete", "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1", []uint64{48, 2039, 97862, 4085603}},
		{"pos3", "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1", []uint64{14, 191, 2812, 43238, 674624}},
		{"pos4", "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1", []uint64{6, 264, 9467, 422333}},
		{"pos5", "rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8", []uint64{44, 1486, 62379, 2103487}},
	}
	for _, c := range cases {
		p, err := ParseFEN(c.fen)
		if err != nil {
			t.Fatalf("%s: ParseFEN: %v", c.name, err)
		}
		for i, want := range c.nodes {
			depth := i + 1
			t.Run(fmt.Sprintf("%s/depth%d", c.name, depth), func(t *testing.T) {
				if testing.Short() && want > shortModeNodeLimit {
					t.Skipf("%d nodes", want)
				}
				if got := perft(p, depth); got != want {
					t.Errorf("perft = %d, want %d", got, want)
				}
			})
		}
	}
}
