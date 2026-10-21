package main

import (
	"reflect"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type actionWithFields struct {
	Kind  game.ActionKind `json:"kind"`
	Text  string          `json:"text"`
	Count int             `json:"count"`
}

func TestActionInputsMustBeStringFieldsOfTheAction(t *testing.T) {
	doc := newDocument(nil, nil)
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

func TestEveryActionKindMaySendOnlyItsKindAndInput(t *testing.T) {
	doc := newDocument(nil, nil)
	ref, err := doc.ref(reflect.TypeFor[actionWithFields]())
	if err != nil {
		t.Fatal(err)
	}
	action := doc.defs[ref.Ref[len(defsPrefix):]]
	action.Required = []string{"kind", "text"}
	for _, tc := range []struct {
		spec    game.ActionSpec
		wantErr bool
	}{
		{game.ActionSpec{Kind: "say", Input: &game.ActionInput{Field: "text", Pattern: "."}}, false},
		{game.ActionSpec{Kind: "stop"}, true},
	} {
		if err := checkRequiredFields(game.Info{Actions: []game.ActionSpec{tc.spec}}, action); (err != nil) != tc.wantErr {
			t.Errorf("action %s: got %v, want error %t", tc.spec.Kind, err, tc.wantErr)
		}
	}
}
