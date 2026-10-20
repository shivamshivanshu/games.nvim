package main

import (
	"reflect"
	"testing"

	"github.com/invopop/jsonschema"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type actionWithFields struct {
	Kind  game.ActionKind `json:"kind"`
	Text  string          `json:"text"`
	Count int             `json:"count"`
}

func TestActionInputsMustBeStringFieldsOfTheAction(t *testing.T) {
	doc := newDocument(nil, func(reflect.Type) *jsonschema.Schema { return nil })
	ref, err := doc.ref(reflect.TypeFor[actionWithFields]())
	if err != nil {
		t.Fatal(err)
	}
	action := doc.defs[ref.Ref[len(defsPrefix):]]
	for field, wantErr := range map[string]bool{"text": false, "count": true, "missing": true} {
		info := game.Info{Actions: []game.ActionSpec{{Kind: "do", Input: &game.ActionInput{Field: field, Pattern: "."}}}}
		if err := checkActionInputs(info, action); (err != nil) != wantErr {
			t.Errorf("field %s: got %v, want error %t", field, err, wantErr)
		}
	}
}
