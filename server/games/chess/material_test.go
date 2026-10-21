package chess

import (
	"reflect"
	"testing"
)

func side(score int, pieces ...PieceLetter) SideMaterial {
	return SideMaterial{Pieces: append([]PieceLetter{}, pieces...), Score: score}
}

func TestMaterialShowsOnlyTheSurplusOfTheSideAheadPerKind(t *testing.T) {
	cases := []struct {
		name string
		fen  string
		want Material
	}{
		{"start position", StartFEN, Material{White: side(0), Black: side(0)}},
		{"white took a knight", "r1bqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 3", Material{White: side(3, "n"), Black: side(0)}},
		{"knights traded", "r1bqkbnr/pppppppp/8/8/8/8/PPPPPPPP/R1BQKBNR w KQkq - 0 5", Material{White: side(0), Black: side(0)}},
		{"white promoted a pawn", "rnbqkbnr/pppppppp/8/8/4Q3/8/PPPPPPP1/RNBQKBNR w KQkq - 0 9", Material{White: side(8, "q"), Black: side(0, "P")}},
		{"each side up something", "1nbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/R2QKBNR w Kk - 0 9", Material{White: side(0, "r"), Black: side(1, "B", "N")}},
	}
	for _, c := range cases {
		if got := mustFEN(t, c.fen).Material(); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestViewCarriesTheMaterialOfThePlayedGame(t *testing.T) {
	cases := []struct {
		name string
		line string
		want Material
	}{
		{"a capture", "e4 d5 exd5", Material{White: side(1, "p"), Black: side(0)}},
		{"a promotion that captures", "a4 b5 axb5 a6 bxa6 Bb7 axb7 Nc6 bxa8=Q", Material{White: side(18, "q", "r", "b", "p"), Black: side(0)}},
	}
	for _, c := range cases {
		st, _ := newGame(t, ColorWhite)
		if got := viewOf(t, playSANs(t, st, c.line)).Material; !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestPlyCountsThePliesBehindAPosition(t *testing.T) {
	cases := map[string]int{
		StartFEN: 0,
		"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1":         1,
		"r1bqk1nr/pppp1ppp/2n5/2b1p3/2B1P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 4 4": 6,
		"r1bqk1nr/pppp1ppp/2n5/2b1p3/2B1P3/5N2/PPPP1PPP/RNBQK2R b KQkq - 4 4": 7,
	}
	for fen, want := range cases {
		if got := mustFEN(t, fen).Ply(); got != want {
			t.Errorf("%s: got %d, want %d", fen, got, want)
		}
	}
}
