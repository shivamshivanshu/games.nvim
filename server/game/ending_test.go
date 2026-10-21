package game_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func TestEndingKeepsTheFirstResultConcluded(t *testing.T) {
	var playing game.Ending
	if _, over := playing.Outcome(); over || playing.Over() {
		t.Fatal("a fresh ending is over")
	}
	first, second := game.WinFor("first", "a"), game.DrawBy("second")
	ended := playing.Concluded(first).Concluded(second)
	if got, over := ended.Outcome(); !over || !ended.Over() || !cmp.Equal(got, first) {
		t.Errorf("Outcome() = %+v, %t; want %+v, true", got, over, first)
	}
	if playing.Over() {
		t.Error("concluding changed the ending it concluded")
	}
}
