package game_test

import (
	"encoding/json"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func TestResultKindEncodesAsItsName(t *testing.T) {
	for kind, name := range map[game.ResultKind]string{game.ResultDecisive: "decisive", game.ResultDraw: "draw", game.ResultAborted: "aborted"} {
		raw, err := json.Marshal(kind)
		if err != nil || string(raw) != `"`+name+`"` {
			t.Errorf("kind %d encodes as %s, %v; want %q", kind, raw, err, name)
		}
		var decoded game.ResultKind
		if err := json.Unmarshal(raw, &decoded); err != nil || decoded != kind {
			t.Errorf("%s decodes as %d, %v; want %d", raw, decoded, err, kind)
		}
	}
	if raw, err := json.Marshal(game.ResultKind(-1)); err == nil {
		t.Errorf("unknown kind encodes as %s", raw)
	}
	var decoded game.ResultKind
	if err := json.Unmarshal([]byte(`"won"`), &decoded); err == nil {
		t.Errorf(`"won" decodes as %d`, decoded)
	}
}
