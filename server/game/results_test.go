package game_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func TestResultConstructorsKeepTheirWinnersShape(t *testing.T) {
	winners := []game.Seat{"a"}
	won := game.WinFor("r", winners...)
	winners[0] = "b"
	for name, tc := range map[string]struct {
		got, want game.Result
	}{
		"a win keeps its winners":            {won, game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{"a"}, Reason: "r"}},
		"a draw has nil winners":             {game.DrawBy("r"), game.Result{Kind: game.ResultDraw, Reason: "r"}},
		"a loss has empty winners":           {game.LossBy("r"), game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{}, Reason: "r"}},
		"a win for nobody has empty winners": {game.WinFor("r"), game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{}, Reason: "r"}},
	} {
		if !cmp.Equal(tc.got, tc.want) || (tc.got.Winners == nil) != (tc.want.Winners == nil) {
			t.Errorf("%s: got %#v, want %#v", name, tc.got, tc.want)
		}
	}
}
