package chess

import (
	"regexp"
	"slices"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func mustFEN(t *testing.T, fen string) Position {
	t.Helper()
	p, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mustParse(t *testing.T, p Position, input string) Move {
	t.Helper()
	m, err := p.ParseMove(input)
	if err != nil {
		t.Fatalf("ParseMove(%q): %v", input, err)
	}
	return m
}

func TestSAN(t *testing.T) {
	tests := []struct{ name, fen, uci, want string }{
		{"file disambiguation", "4k3/8/8/8/8/5N2/8/1N2K3 w - - 0 1", "b1d2", "Nbd2"},
		{"rank disambiguation", "4k3/8/8/R7/8/8/8/R3K3 w - - 0 1", "a1a3", "R1a3"},
		{"file and rank disambiguation", "8/k7/8/8/4Q2Q/8/8/K6Q w - - 0 1", "h4e1", "Qh4e1"},
		{"en passant", "4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1", "e5d6", "exd6"},
		{"promotion", "8/4P1k1/8/8/8/8/8/4K3 w - - 0 1", "e7e8q", "e8=Q"},
		{"kingside castle", "4k3/8/8/8/8/8/8/4K2R w K - 0 1", "e1g1", "O-O"},
		{"queenside castle", "4k3/8/8/8/8/8/8/R3K3 w Q - 0 1", "e1c1", "O-O-O"},
		{"checkmate", "rnbqkbnr/pppp1ppp/8/4p3/6P1/5P2/PPPPP2P/RNBQKBNR b KQkq g3 0 2", "d8h4", "Qh4#"},
		{"check", "rnbqkbnr/ppp2ppp/8/3pp3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 3", "f1b5", "Bb5+"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := mustFEN(t, tc.fen)
			if got := p.SAN(mustParse(t, p, tc.uci)); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestParseMoveVariants(t *testing.T) {
	start := mustFEN(t, StartFEN)
	e4 := Move{From: mustParseSquare("e2"), To: mustParseSquare("e4")}
	for _, input := range []string{" e4 ", "E2E4", "e2e4", "e4!"} {
		if got := mustParse(t, start, input); got != e4 {
			t.Errorf("%q: got %v want %v", input, got, e4)
		}
	}
	if got, want := mustParse(t, start, "Nf3+"), (Move{From: mustParseSquare("g1"), To: mustParseSquare("f3")}); got != want {
		t.Errorf("Nf3+: got %v want %v", got, want)
	}

	castle := mustFEN(t, "4k3/8/8/8/8/8/8/4K2R w K - 0 1")
	if got := mustParse(t, castle, "0-0"); got.UCI() != "e1g1" {
		t.Errorf("0-0: got %v", got)
	}
	castleLong := mustFEN(t, "4k3/8/8/8/8/8/8/R3K3 w Q - 0 1")
	if got := mustParse(t, castleLong, "0-0-0"); got.UCI() != "e1c1" {
		t.Errorf("0-0-0: got %v", got)
	}

	promo := mustFEN(t, "8/4P1k1/8/8/8/8/8/4K3 w - - 0 1")
	if a, b := mustParse(t, promo, "e8Q"), mustParse(t, promo, "e8=Q"); a != b || a.Promotion != Queen {
		t.Errorf("e8Q=%v e8=Q=%v", a, b)
	}
}

func TestParseMoveCaseInsensitive(t *testing.T) {
	const (
		pieces    = "4k3/8/8/3p4/8/8/8/R2QKBNR w - - 0 1"
		promotion = "8/4P1k1/8/8/8/8/8/4K3 w - - 0 1"
		castling  = "4k3/8/8/8/8/8/8/R3K2R w KQ - 0 1"
		collision = "4k3/8/8/8/2p5/1P6/8/4KB2 w - - 0 1"
	)
	tests := []struct{ name, fen, input, wantUCI string }{
		{"knight", pieces, "nf3", "g1f3"},
		{"bishop without pawn", pieces, "bc4", "f1c4"},
		{"rook", pieces, "ra7", "a1a7"},
		{"queen", pieces, "qd3", "d1d3"},
		{"king", pieces, "ke2", "e1e2"},
		{"capture", pieces, "qxd5", "d1d5"},
		{"check", pieces, "ra8+", "a1a8"},
		{"upper case", pieces, "NF3", "g1f3"},
		{"promotion with equals", promotion, "e8=q", "e7e8q"},
		{"promotion without equals", promotion, "e8n", "e7e8n"},
		{"kingside castle", castling, "o-o", "e1g1"},
		{"queenside castle", castling, "o-o-o", "e1c1"},
		{"exact pawn capture wins", collision, "bxc4", "b3c4"},
		{"exact bishop capture wins", collision, "Bxc4", "f1c4"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := mustParse(t, mustFEN(t, tc.fen), tc.input).UCI(); got != tc.wantUCI {
				t.Errorf("got %s want %s", got, tc.wantUCI)
			}
		})
	}

	_, err := mustFEN(t, collision).ParseMove("BXC4")
	if want := "ambiguous move: BXC4 (Bxc4 or bxc4)"; err == nil || err.Error() != want {
		t.Errorf("got %v want %s", err, want)
	}
}

func historyAfter(t *testing.T, p Position, sans ...string) (Position, []RepetitionKey) {
	t.Helper()
	history := []RepetitionKey{p.RepetitionKey()}
	for _, san := range sans {
		p = p.Play(mustParse(t, p, san))
		history = append(history, p.RepetitionKey())
	}
	return p, history
}

func detectEnding(p Position, history []RepetitionKey) (Ending, bool) {
	return DetectEnding(p, p.LegalMoves(), p.RepetitionKey(), history)
}

func TestInsufficientMaterial(t *testing.T) {
	tests := []struct {
		name, fen string
		want      bool
	}{
		{"K v K", "4k3/8/8/8/8/8/8/4K3 w - - 0 1", true},
		{"KB v K", "4k3/8/8/8/8/8/8/2B1K3 w - - 0 1", true},
		{"KN v K", "4k3/8/8/8/8/8/8/1N2K3 w - - 0 1", true},
		{"KB v KB same colour", "4kb2/8/8/8/8/8/8/2B1K3 w - - 0 1", true},
		{"KB v KB opposite colour", "4k1b1/8/8/8/8/8/8/2B1K3 w - - 0 1", false},
		{"KNN v K", "4k3/8/8/8/8/8/8/1NN1K3 w - - 0 1", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := mustFEN(t, tc.fen)
			ending, got := detectEnding(p, []RepetitionKey{p.RepetitionKey()})
			if got != tc.want || (got && ending.Reason != ReasonInsufficientMaterial) {
				t.Errorf("got %+v, %t want %t", ending, got, tc.want)
			}
		})
	}
}

func TestSeventyFiveAndFiftyMove(t *testing.T) {
	p, history := historyAfter(t, mustFEN(t, "4k3/8/8/8/8/8/8/R3K3 w - - 149 80"), "Ra2")
	if ending, ok := detectEnding(p, history); !ok || ending.Reason != ReasonSeventyFive || ending.Kind != game.ResultDraw {
		t.Errorf("75-move: got %+v, %t", ending, ok)
	}
	p, history = historyAfter(t, mustFEN(t, "4k3/8/8/8/8/8/8/R3K3 w - - 99 80"), "Ra2")
	if _, ok := detectEnding(p, history); ok {
		t.Error("100 halfmoves must not end the game automatically")
	}
	if reason, ok := ClaimableDraw(p, history); !ok || reason != ReasonFifty {
		t.Errorf("50-move: got %q, %t", reason, ok)
	}
}

func TestClaimableDrawCountsOnlyThisPositionsKey(t *testing.T) {
	p := mustFEN(t, StartFEN)
	for name, history := range map[string][]RepetitionKey{"empty": nil, "other position": {"A", "A", "A"}} {
		if reason, ok := ClaimableDraw(p, history); ok {
			t.Errorf("%s history: claim %q", name, reason)
		}
	}
	if reason, ok := ClaimableDraw(p, slices.Repeat([]RepetitionKey{p.RepetitionKey()}, threefoldCount)); !ok || reason != ReasonThreefold {
		t.Errorf("thrice repeated: got %q, %t", reason, ok)
	}
}

func TestFivefoldAutomatic(t *testing.T) {
	shuffle := []string{"Nf3", "Nf6", "Ng1", "Ng8"}
	p, history := historyAfter(t, mustFEN(t, StartFEN), slices.Concat(shuffle, shuffle, shuffle)...)
	if _, ok := detectEnding(p, history); ok {
		t.Fatal("threefold must not end the game automatically")
	}
	p, history = historyAfter(t, mustFEN(t, StartFEN), slices.Concat(shuffle, shuffle, shuffle, shuffle)...)
	if ending, ok := detectEnding(p, history); !ok || ending.Reason != ReasonFivefold {
		t.Errorf("got %+v, %t", ending, ok)
	}
}

func TestMovePatternMatchesWhatParseMoveReads(t *testing.T) {
	pattern := regexp.MustCompile(movePattern)
	start := mustFEN(t, StartFEN)
	readable := []string{" e4 ", "E2E4", "e4!", "Nf3+", "nf3", "NF3", "qxd5", "e8=q", "e8Q", "e7e8n", "0-0", "o-o-o", "O-O-O", "Qh4#", "Qh4e1", "R1a3", "exd6"}
	for _, m := range start.LegalMoves() {
		readable = append(readable, m.UCI(), start.SAN(m))
	}
	for _, input := range readable {
		if !pattern.MatchString(input) {
			t.Errorf("pattern rejects %q", input)
		}
	}
	for _, input := range []string{"", "e9", "hello", "Kz1", "e2-e4"} {
		if pattern.MatchString(input) {
			t.Errorf("pattern accepts %q", input)
		}
	}
}
