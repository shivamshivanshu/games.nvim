package main

import (
	"maps"
	"reflect"
	"slices"

	"github.com/invopop/jsonschema"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
)

const protocolFile = "protocol.json"

// messageSchemas is one entry of x-messages: a request's body and its ok
// reply's body, or a push's body.
type messageSchemas struct {
	Request *jsonschema.Schema `json:"request,omitempty"`
	Reply   *jsonschema.Schema `json:"reply,omitempty"`
	Push    *jsonschema.Schema `json:"push,omitempty"`
}

// protocolSchema validates one envelope; x-messages gives the body schemas of
// each message type. Its enums are the core's, which the games' schemas refer to.
func protocolSchema(comments map[string]string) (*jsonschema.Schema, []reflect.Type, error) {
	doc := newDocument(comments, nil)
	envelope, err := doc.ref(reflect.TypeFor[protocol.Envelope]())
	if err != nil {
		return nil, nil, err
	}
	if _, err := doc.ref(reflect.TypeFor[protocol.ErrorBody]()); err != nil {
		return nil, nil, err
	}
	messages := map[protocol.MessageType]messageSchemas{}
	for _, r := range protocol.Requests {
		var m messageSchemas
		if m.Request, err = doc.ref(r.Body); err != nil {
			return nil, nil, err
		}
		if m.Reply, err = doc.ref(r.Reply); err != nil {
			return nil, nil, err
		}
		messages[r.Type] = m
	}
	for _, p := range protocol.Pushes {
		var m messageSchemas
		if m.Push, err = doc.ref(p.Body); err != nil {
			return nil, nil, err
		}
		messages[p.Type] = m
	}
	if err := doc.defineEnums(); err != nil {
		return nil, nil, err
	}
	return &jsonschema.Schema{
		Version:     jsonschema.Version,
		Title:       "games.nvim protocol",
		Description: "One NDJSON line. x-messages maps each message type to the schemas of its body: a request's, its ok reply's, or a push's.",
		Ref:         envelope.Ref,
		Definitions: doc.defs,
		Extras:      map[string]any{"x-version": protocol.Version, "x-messages": messages},
	}, slices.Collect(maps.Keys(doc.enums)), nil
}
