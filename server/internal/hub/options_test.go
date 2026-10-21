package hub_test

import (
	"reflect"
	"strings"
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

// embeddingGame is the counter with an action type that embeds a struct.
type embeddingGame struct{ game.Game }

type embeddingAction struct{ gametest.CounterAction }

func (embeddingGame) WireTypes() (action, view reflect.Type) {
	return reflect.TypeFor[embeddingAction](), reflect.TypeFor[gametest.CounterView]()
}

func TestNewRejectsInvalidGameInfo(t *testing.T) {
	t.Parallel()
	counter := game.Adapt(gametest.CounterRules{})
	tests := map[string]struct {
		games   []game.Game
		wantErr string
	}{
		"an option default its own option rejects": {[]game.Game{game.Adapt(badDefaultRules{})}, "note"},
		"a game type registered twice":             {[]game.Game{counter, counter}, "registered twice"},
		"an action type that embeds a struct":      {[]game.Game{embeddingGame{counter}}, "embeds gametest.CounterAction"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			_, err := hub.New(t.Context(), hub.Deps{Store: e.store, Games: tc.games, Limits: limits.Default(), Time: e.clock})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got %v, want an error naming %q", err, tc.wantErr)
			}
		})
	}
}
