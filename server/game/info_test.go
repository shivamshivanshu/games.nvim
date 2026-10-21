package game_test

import (
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

type otherEvent struct{}

func (otherEvent) EventKind() game.EventKind { return "other" }

// validInfo is the smallest Info that passes Check, after edit.
func validInfo(edit func(*game.Info)) game.Info {
	info := game.Info{Type: "stub", Actions: []game.ActionSpec{{Kind: "pass"}}, Events: []game.Event{gametest.CountedEvent{}}}
	edit(&info)
	return info
}

func TestInfoCheck(t *testing.T) {
	valid := validInfo
	clocked := []game.Capability{game.CapResign, game.CapClock}
	for name, tc := range map[string]struct {
		info    game.Info
		wantErr bool
	}{
		"an action and an event":         {valid(func(*game.Info) {}), false},
		"no events":                      {valid(func(i *game.Info) { i.Events = nil }), true},
		"distinct events":                {valid(func(i *game.Info) { i.Events = []game.Event{gametest.CountedEvent{}, otherEvent{}} }), false},
		"a duplicate event kind":         {valid(func(i *game.Info) { i.Events = []game.Event{gametest.CountedEvent{}, gametest.CountedEvent{}} }), true},
		"a clock with a time control":    {valid(func(i *game.Info) { i.Capabilities, i.Options = clocked, []game.OptionSpec{game.TimeControlOption()} }), false},
		"a clock without a time control": {valid(func(i *game.Info) { i.Capabilities = clocked }), true},
		"a time control without a clock": {valid(func(i *game.Info) { i.Options = []game.OptionSpec{game.TimeControlOption()} }), true},
		"no actions":                     {valid(func(i *game.Info) { i.Actions = nil }), true},
		"an invalid option":              {valid(func(i *game.Info) { i.Options = []game.OptionSpec{{Name: "e", Default: "z"}} }), true},
	} {
		if err := tc.info.Check(); (err != nil) != tc.wantErr {
			t.Errorf("%s: got %v, want an error %t", name, err, tc.wantErr)
		}
	}
}
