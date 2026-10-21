package hub_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
)

// sentAction is a game.act a transcript sends, keyed by its client and id
// until the reply to it comes.
type sentAction struct {
	at     string
	game   string
	action any
}

type requestKey struct {
	client string
	id     string
}

// The conformance runner checks what the server sends; this checks what the
// transcripts send it, so every action the server takes is one its game's
// schema allows.
func TestEveryActionTheServerTakesMatchesItsGameSchema(t *testing.T) {
	t.Parallel()
	w := sharedWireSchema(t)
	paths, err := filepath.Glob(filepath.Join(conformanceDir, "*.ndjson"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no transcripts in %s: %v", conformanceDir, err)
	}
	checked := 0
	for _, path := range paths {
		lines := readTranscript(t, path)
		games := gamesByID(t, lines)
		pending := map[requestKey]sentAction{}
		for n, line := range lines {
			at := fmt.Sprintf("%s:%d", filepath.Base(path), n+1)
			switch {
			case line.Send != nil:
				msg := decodeSchemaValue(t, at, line.Send)
				if msg["type"] != string(protocol.TypeGameAct) {
					continue
				}
				body := msg["body"].(map[string]any)
				gameID, _ := body["game_id"].(string)
				pending[requestKey{line.Client, fmt.Sprint(msg["id"])}] = sentAction{at: at, game: games[gameID], action: body["action"]}
			case line.Recv != nil:
				msg := decodeSchemaValue(t, at, line.Recv)
				key := requestKey{line.Client, fmt.Sprint(msg["id"])}
				sent, ok := pending[key]
				if !ok {
					continue
				}
				delete(pending, key)
				if msg["type"] != string(protocol.TypeOK) || sent.game == unschemaedGame {
					continue
				}
				if err := w.checkAction(sent.game, sent.action); err != nil {
					t.Errorf("%s: the server took an action its schema refuses: %v", sent.at, err)
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Error("no transcript action was checked")
	}
}

// gamesByID maps each game id the transcript lines name to its game type.
func gamesByID(t *testing.T, lines []transcriptLine) map[string]string {
	games := map[string]string{}
	var collect func(v any)
	collect = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			id, hasID := v["game_id"].(string)
			game, hasGame := v["game"].(string)
			if hasID && hasGame {
				games[id] = game
			}
			for _, child := range v {
				collect(child)
			}
		case []any:
			for _, child := range v {
				collect(child)
			}
		}
	}
	for n, line := range lines {
		if line.Recv != nil {
			collect(decodeSchemaValue(t, fmt.Sprint(n+1), line.Recv))
		}
	}
	return games
}

func decodeSchemaValue(t *testing.T, at string, raw json.RawMessage) map[string]any {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("%s: %v", at, err)
	}
	return v.(map[string]any)
}

// patternSamples are, per game and kind, inputs its pattern takes, in each
// case it takes them, and one it refuses.
var patternSamples = map[string]map[game.ActionKind]struct {
	valid   []string
	invalid string
}{
	"battleship": {
		"place_across": {[]string{"a1", "J10"}, "k1"},
		"place_down":   {[]string{"a1", "J10"}, "k1"},
		"fire":         {[]string{"c5", "C5"}, "a11"},
	},
	"checkers":       {"move": {[]string{"11-15", "15x24x31"}, "33-1"}},
	"chess":          {"move": {[]string{"e4", "Nf3", "O-O", "exd8=Q", "E2E4"}, "e9"}},
	"chess960":       {"move": {[]string{"e4", "f1h1", "F1H1"}, "i1"}},
	"chess_puzzle":   {"move": {[]string{"nf3", "E2E4"}, "e9"}},
	"dots_and_boxes": {"draw": {[]string{"b2h", "B2V"}, "b2"}},
	"fifteen":        {"slide": {[]string{"7", " 15 "}, "16"}},
	"gomoku":         {"place": {[]string{"h8", "O15"}, "p1"}},
	"hangman":        {"guess": {[]string{"a", "CRANE"}, "ab"}},
	"hearts": {
		"pass": {[]string{"2c qs ah", "2C QS AH"}, "2c qs"},
		"play": {[]string{"qs", "TD"}, "1s"},
	},
	"liars_dice": {"bid": {[]string{"3x5", "12X6"}, "3x7"}},
	"mastermind": {"guess": {[]string{"rgby", "OPRG"}, "rgbw"}},
	"minesweeper": {
		"reveal": {[]string{"c12", "P30"}, "q1"},
		"flag":   {[]string{"c12", "P30"}, "q1"},
		"chord":  {[]string{"c12", "P30"}, "q1"},
	},
	"poker":   {"act": {[]string{"fold", "Check", "CALL", "allin", "bet 40", "RAISE 400"}, "bet +20"}},
	"reversi": {"place": {[]string{"d3", "H8"}, "i1"}},
	"sudoku":  {"set": {[]string{"53 7", " 11 0 "}, "5 3 7"}, "clear": {[]string{"53", "99"}, "53 0"}},
	"uno":     {"play": {[]string{"r5", "G+2", "ys", "w b", "W+4 Y"}, "x5"}},
	"wordle":  {"guess": {[]string{"crane", "CRANE"}, "cran"}},
}

// Each catalogued kind's oneOf branch takes the kind with each of its values,
// or the samples of its pattern, and refuses an input off them, a missing
// input, a field it does not take, and an unknown kind.
func TestEveryCataloguedKindMatchesItsActionSchema(t *testing.T) {
	t.Parallel()
	w := sharedWireSchema(t)
	for _, g := range games.All() {
		info := g.Info()
		check := func(action map[string]any, wantValid bool) {
			if err := w.checkAction(string(info.Type), action); (err == nil) != wantValid {
				t.Errorf("%s %v: valid %t, want %t: %v", info.Type, action, err == nil, wantValid, err)
			}
		}
		for _, spec := range info.Actions {
			kind := string(spec.Kind)
			check(map[string]any{"kind": kind, "unasked": "x"}, false)
			if spec.Input == nil {
				check(map[string]any{"kind": kind}, true)
				continue
			}
			field := spec.Input.Field
			check(map[string]any{"kind": kind}, false)
			valid, invalid := spec.Input.Values, "no such value"
			if spec.Input.Pattern != "" {
				samples, ok := patternSamples[string(info.Type)][spec.Kind]
				if !ok {
					t.Errorf("%s %s: no pattern samples", info.Type, kind)
					continue
				}
				valid, invalid = samples.valid, samples.invalid
			}
			for _, text := range valid {
				check(map[string]any{"kind": kind, field: text}, true)
			}
			check(map[string]any{"kind": kind, field: invalid}, false)
		}
		check(map[string]any{"kind": "no_such_kind"}, false)
	}
}
