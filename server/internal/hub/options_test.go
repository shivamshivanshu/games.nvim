package hub_test

import (
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/internal/hub"
	"github.com/shivamshivanshu/games.nvim/server/internal/limits"
)

type badDefaultRules struct{ gametest.CounterRules }

func (badDefaultRules) Info() game.Info {
	info := gametest.CounterRules{}.Info()
	info.Options = []game.OptionSpec{game.TextOption("note", 2)}
	info.Options[0].Default = "too long"
	return info
}

func TestNewRejectsGameWithInvalidOptionDefault(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, err := hub.New(t.Context(), hub.Deps{
		Store: e.store, Games: []game.Game{game.Adapt(badDefaultRules{})}, Limits: limits.Default(), Time: e.clock,
	})
	if err == nil {
		t.Fatal("want an error for a default its own option rejects")
	}
}
