package main

import (
	"errors"
	"fmt"
	"path"
	"reflect"
	"strings"

	"github.com/invopop/jsonschema"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

const defsPrefix = "#/$defs/"

func gameFile(t game.GameType) string { return path.Join("games", string(t)+".json") }

// gameSchema has the game's Action, View and events in its $defs, pointed to
// by x-action, x-view and x-events; the core's enums stay in the protocol schema.
func gameSchema(g game.Game, comments map[string]string) (*jsonschema.Schema, error) {
	info := g.Info()
	eventCatalog, ok := g.EventCatalog()
	if !ok {
		return nil, errors.New("its rules implement no game.EventCatalog")
	}
	actionKinds := closed(xslices.Map(info.Actions, func(a game.ActionSpec) game.ActionKind { return a.Kind })...)
	local, core := enumRefs([]enum{actionKinds}, ""), enumRefs(coreEnums(), "../"+protocolFile)
	doc := newDocument(comments, func(t reflect.Type) *jsonschema.Schema {
		if s := local(t); s != nil {
			return s
		}
		return core(t)
	})
	actionType, viewType := g.WireTypes()
	action, err := doc.ref(actionType)
	if err != nil {
		return nil, err
	}
	if err := checkActionInputs(info, doc.defs[strings.TrimPrefix(action.Ref, defsPrefix)]); err != nil {
		return nil, err
	}
	view, err := doc.ref(viewType)
	if err != nil {
		return nil, err
	}
	events := map[game.EventKind]*jsonschema.Schema{}
	for _, e := range eventCatalog.EventTypes() {
		if events[e.EventKind()], err = doc.eventRef(e); err != nil {
			return nil, err
		}
	}
	if err := doc.defineEnum(actionKinds); err != nil {
		return nil, err
	}
	if err := defineReasons(doc, g, info.Name); err != nil {
		return nil, err
	}
	return &jsonschema.Schema{
		Version:     jsonschema.Version,
		Title:       info.Name,
		Description: "The game-specific bodies inside a " + string(info.Type) + " game.update and game.act.",
		Definitions: doc.defs,
		Extras:      map[string]any{"x-action": action, "x-view": view, "x-events": events},
	}, nil
}

// checkActionInputs fails on a catalogued input whose field is not a string
// property of the action.
func checkActionInputs(info game.Info, action *jsonschema.Schema) error {
	for _, spec := range info.Actions {
		if spec.Input == nil {
			continue
		}
		var property *jsonschema.Schema
		if action != nil && action.Properties != nil {
			property, _ = action.Properties.Get(spec.Input.Field)
		}
		if property == nil || property.Type != "string" {
			return fmt.Errorf("action %q takes input %q, which is not a string field of its action", spec.Kind, spec.Input.Field)
		}
	}
	return nil
}

// defineReasons adds the ResultReason enum of a game whose rules are a
// non-empty game.ReasonCatalog.
func defineReasons(doc *document, g game.Game, name string) error {
	catalog, ok := g.ReasonCatalog()
	if !ok || len(catalog.Reasons()) == 0 {
		return nil
	}
	reasons := closed(catalog.Reasons()...)
	return doc.define(reasons.typ.Name(), reasons.schema(fmt.Sprintf("The result reasons %s adds to the core's.", name)))
}
