package hub_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
)

const schemaDir = "../../../docs/schema"

// wireSchema checks server messages against the checked-in JSON Schema.
type wireSchema struct {
	dir      string
	compiler *jsonschema.Compiler
	compiled map[string]*jsonschema.Schema
	messages map[protocol.MessageType]map[string]schemaRef
	games    map[string]gameSchemaRefs
}

type schemaRef struct {
	Ref string `json:"$ref"`
}

type gameSchemaRefs struct {
	View   schemaRef            `json:"x-view"`
	Events map[string]schemaRef `json:"x-events"`
}

func loadWireSchema(t *testing.T) *wireSchema {
	t.Helper()
	dir, err := filepath.Abs(schemaDir)
	if err != nil {
		t.Fatal(err)
	}
	var proto struct {
		Messages map[protocol.MessageType]map[string]schemaRef `json:"x-messages"`
	}
	readJSON(t, filepath.Join(dir, "protocol.json"), &proto)
	w := &wireSchema{dir: dir, compiler: jsonschema.NewCompiler(), compiled: map[string]*jsonschema.Schema{}, messages: proto.Messages, games: map[string]gameSchemaRefs{}}
	paths, err := filepath.Glob(filepath.Join(dir, "games", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		var refs gameSchemaRefs
		readJSON(t, path, &refs)
		w.games[filepath.Base(path[:len(path)-len(".json")])] = refs
	}
	return w
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func (w *wireSchema) validate(file, ref string, v any) error {
	loc := filepath.Join(w.dir, file) + ref
	s, ok := w.compiled[loc]
	if !ok {
		var err error
		if s, err = w.compiler.Compile(loc); err != nil {
			return fmt.Errorf("compile %s: %w", loc, err)
		}
		w.compiled[loc] = s
	}
	return s.Validate(v)
}

// check validates msg, a line the server sent: its envelope, its body as the
// schema of its type gives it, and any game's view and events inside it.
// requestType is the type of the request msg replies to.
func (w *wireSchema) check(msg map[string]any, requestType protocol.MessageType) error {
	if err := w.validate("protocol.json", "", msg); err != nil {
		return err
	}
	typ, body := protocol.MessageType(msg["type"].(string)), msg["body"]
	var ref string
	switch typ {
	case protocol.TypeOK:
		ref, typ = w.messages[requestType]["reply"].Ref, requestType
	case protocol.TypeError:
		ref = "#/$defs/ErrorBody"
	default:
		ref = w.messages[typ]["push"].Ref
	}
	if ref == "" {
		return fmt.Errorf("no schema for a %s body", typ)
	}
	if err := w.validate("protocol.json", ref, body); err != nil {
		return err
	}
	for _, update := range gameUpdatesIn(typ, body) {
		if err := w.checkGame(update); err != nil {
			return err
		}
	}
	return nil
}

func gameUpdatesIn(typ protocol.MessageType, body any) []any {
	switch typ {
	case protocol.TypeGameUpdate, protocol.TypeGameSnapshot, protocol.TypeGameStart:
		return []any{body}
	case protocol.TypeGameList:
		return body.(map[string]any)["games"].([]any)
	}
	return nil
}

// unschemaedGame is the one game the replay server registers that has no
// schema: a test game, so that a solo start is covered.
const unschemaedGame = "solo_counter"

// checkGame validates the view and events of update, failing for a game
// without a schema.
func (w *wireSchema) checkGame(update any) error {
	u := update.(map[string]any)
	game := u["game"].(string)
	refs, ok := w.games[game]
	switch {
	case !ok && game == unschemaedGame:
		return nil
	case !ok:
		return fmt.Errorf("game %s has no schema", game)
	}
	file := filepath.Join("games", game+".json")
	if err := w.validate(file, refs.View.Ref, u["view"]); err != nil {
		return err
	}
	for _, e := range u["events"].([]any) {
		kind, _ := e.(map[string]any)["kind"].(string)
		ref, ok := refs.Events[kind]
		if !ok {
			return fmt.Errorf("%s event kind %q has no schema", game, kind)
		}
		if err := w.validate(file, ref.Ref, e); err != nil {
			return err
		}
	}
	return nil
}
