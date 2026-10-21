package main

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type actionWithFields struct {
	Kind  game.ActionKind `json:"kind"`
	Text  string          `json:"text"`
	Count int             `json:"count"`
}

func TestActionOneOfFailsOnAnInputItCannotCarry(t *testing.T) {
	doc := newDocument(nil, nil)
	ref, err := doc.ref(reflect.TypeFor[actionWithFields]())
	if err != nil {
		t.Fatal(err)
	}
	action := doc.defs[ref.Ref[len(defsPrefix):]]
	say := game.ActionSpec{Kind: "say", Input: &game.ActionInput{Field: "text", Pattern: "."}}
	for _, tc := range []struct {
		name    string
		specs   []game.ActionSpec
		wantErr bool
	}{
		{"int input", []game.ActionSpec{say, {Kind: "count", Input: &game.ActionInput{Field: "count", Values: []string{"1"}}}}, true},
		{"missing input", []game.ActionSpec{say, {Kind: "ask", Input: &game.ActionInput{Field: "missing", Pattern: "."}}}, true},
		{"field no kind takes", []game.ActionSpec{say}, true},
	} {
		if _, err := actionOneOf(game.Info{Actions: tc.specs}, action); (err != nil) != tc.wantErr {
			t.Errorf("%s: got %v, want error %t", tc.name, err, tc.wantErr)
		}
	}
}

type actionWithText struct {
	Kind game.ActionKind `json:"kind"`
	Text string          `json:"text"`
}

func TestActionOneOfHasEachKindWithOnlyItsInput(t *testing.T) {
	doc := newDocument(nil, nil)
	ref, err := doc.ref(reflect.TypeFor[actionWithText]())
	if err != nil {
		t.Fatal(err)
	}
	info := game.Info{Actions: []game.ActionSpec{
		{Kind: "say", Input: &game.ActionInput{Field: "text", Pattern: "^a$"}},
		{Kind: "stop"},
	}}
	oneOf, err := actionOneOf(info, doc.defs[ref.Ref[len(defsPrefix):]])
	if err != nil {
		t.Fatal(err)
	}
	say, stop := oneOf.OneOf[0], oneOf.OneOf[1]
	if kind, _ := say.Properties.Get("kind"); kind.Const != "say" || len(say.Required) != 2 {
		t.Errorf("say: kind %+v, required %v", kind, say.Required)
	}
	if text, _ := say.Properties.Get("text"); text == nil || text.Pattern != "^a$" {
		t.Errorf("say: text %+v", text)
	}
	for _, alternative := range oneOf.OneOf {
		if closed, err := json.Marshal(alternative.AdditionalProperties); err != nil || string(closed) != "false" {
			t.Errorf("%v takes fields it does not list: additionalProperties %s", alternative.Required, closed)
		}
	}
	if _, has := stop.Properties.Get("text"); has {
		t.Errorf("stop takes more than its kind: %+v", stop)
	}
}
