package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

const (
	defsPrefix      = "#/$defs/"
	actionKindField = "kind"
)

func gameFile(t game.GameType) string { return path.Join("games", string(t)+".json") }

// gameSchema has the game's Action, View and events in its $defs, pointed to
// by x-action, x-view and x-events; the core's enums stay in the protocol schema.
func gameSchema(g game.Game, comments map[string]string, coreEnums []reflect.Type) (*jsonschema.Schema, error) {
	info := g.Info()
	if err := info.Check(); err != nil {
		return nil, err
	}
	actionKinds := closed(xslices.Map(info.Actions, func(a game.ActionSpec) game.ActionKind { return a.Kind })...)
	refs := xslices.With(enumRefs(coreEnums, "../"+protocolFile), actionKinds.typ, defsPrefix+actionKinds.typ.Name())
	doc := newDocument(comments, refs)
	actionType, viewType := g.WireTypes()
	action, err := doc.ref(actionType)
	if err != nil {
		return nil, err
	}
	view, err := doc.ref(viewType)
	if err != nil {
		return nil, err
	}
	events := map[game.EventKind]*jsonschema.Schema{}
	for _, e := range info.Events {
		if events[e.EventKind()], err = doc.eventRef(e); err != nil {
			return nil, err
		}
	}
	if err := doc.defineEnums(); err != nil {
		return nil, err
	}
	actionName := strings.TrimPrefix(action.Ref, defsPrefix)
	if doc.defs[actionName], err = actionOneOf(info, doc.defs[actionName]); err != nil {
		return nil, err
	}
	if err := defineReferredEnum(doc, actionKinds); err != nil {
		return nil, err
	}
	if err := defineReasons(doc, info); err != nil {
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

// actionOneOf is one alternative of action per catalogued kind: the kind,
// and the input the kind takes, as its pattern or values restrict it. It
// fails on an input that is not a string field of action, and on a field of
// action no kind takes.
func actionOneOf(info game.Info, action *jsonschema.Schema) (*jsonschema.Schema, error) {
	taken := map[string]bool{actionKindField: true}
	alternatives := make([]*jsonschema.Schema, 0, len(info.Actions))
	for _, spec := range info.Actions {
		alternative, err := actionAlternative(spec, action)
		if err != nil {
			return nil, err
		}
		if spec.Input != nil {
			taken[spec.Input.Field] = true
		}
		alternatives = append(alternatives, alternative)
	}
	for p := action.Properties.Oldest(); p != nil; p = p.Next() {
		if !taken[p.Key] {
			return nil, fmt.Errorf("no action takes the action field %q", p.Key)
		}
	}
	return &jsonschema.Schema{Description: action.Description, OneOf: alternatives}, nil
}

func actionAlternative(spec game.ActionSpec, action *jsonschema.Schema) (*jsonschema.Schema, error) {
	properties := orderedmap.New[string, *jsonschema.Schema]()
	properties.Set(actionKindField, &jsonschema.Schema{Const: string(spec.Kind)})
	required := []string{actionKindField}
	if in := spec.Input; in != nil {
		field, _ := action.Properties.Get(in.Field)
		if field == nil || field.Type != "string" {
			return nil, fmt.Errorf("action %q takes input %q, which is not a string field of its action", spec.Kind, in.Field)
		}
		properties.Set(in.Field, &jsonschema.Schema{
			Type:        "string",
			Description: in.Description,
			Pattern:     in.Pattern,
			Enum:        xslices.Map(in.Values, func(v string) any { return v }),
		})
		required = append(required, in.Field)
	}
	return &jsonschema.Schema{
		Type:                 "object",
		Description:          spec.Description,
		Properties:           properties,
		Required:             required,
		AdditionalProperties: jsonschema.FalseSchema,
	}, nil
}

// defineReferredEnum adds e's def if a def refers to it, as a view listing
// action kinds does; the actions' alternatives hold their kinds as consts.
func defineReferredEnum(doc *document, e enum) error {
	encoded, err := json.Marshal(doc.defs)
	if err != nil || !bytes.Contains(encoded, []byte(strconv.Quote(defsPrefix+e.typ.Name()))) {
		return err
	}
	return doc.defineEnum(e)
}

// defineReasons adds the ResultReason enum of a game that declares reasons.
func defineReasons(doc *document, info game.Info) error {
	if len(info.Reasons) == 0 {
		return nil
	}
	reasons := closed(info.Reasons...)
	return doc.define(reasons.typ.Name(), reasons.schema(fmt.Sprintf("The result reasons %s adds to the core's.", info.Name)))
}
