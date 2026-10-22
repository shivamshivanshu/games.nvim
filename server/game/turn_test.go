package game_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func TestTurn(t *testing.T) {
	won := game.WinFor("won", "black")
	imposed := game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	tests := []struct {
		name       string
		turn       game.Turn
		wantToAct  []game.Seat
		wantResult game.Result
	}{
		{"black first", game.TurnTo("black"), []game.Seat{"black"}, game.Result{}},
		{"passed to white", game.TurnTo("black").To("white"), []game.Seat{"white"}, game.Result{}},
		{"concluded", game.TurnTo("black").Conclude(imposed), nil, imposed},
		{"a reached result outlives a conclusion", game.TurnTo("black").Conclude(won).Conclude(imposed), nil, won},
		{"a finished turn passes to no one", game.TurnTo("black").Conclude(won).To("white"), nil, won},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, finished := tc.turn.Outcome()
			if finished != (tc.wantToAct == nil) || !cmp.Equal(tc.wantResult, result) {
				t.Errorf("outcome %+v, %t", result, finished)
			}
			if diff := cmp.Diff(tc.wantToAct, tc.turn.ToAct()); diff != "" {
				t.Errorf("to act (-want +got):\n%s", diff)
			}
		})
	}
}
