package game_test

import (
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func TestUnknownActionNamesTheKind(t *testing.T) {
	if err := game.UnknownAction(game.ActionKind("jump")); err != (game.IllegalActionError{Msg: "unknown action: jump"}) {
		t.Errorf("UnknownAction = %#v", err)
	}
}
