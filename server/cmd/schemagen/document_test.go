package main

import (
	"reflect"
	"testing"

	"github.com/invopop/jsonschema"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type kindedEvent struct {
	Kind game.EventKind `json:"-"`
	By   string         `json:"by"`
}

func (e kindedEvent) EventKind() game.EventKind { return e.Kind }

type viewHoldingEvent struct {
	Last kindedEvent `json:"last"`
}

func TestEventKindIsNeverAddedToASharedDef(t *testing.T) {
	doc := newDocument(nil, func(reflect.Type) *jsonschema.Schema { return nil })
	if _, err := doc.ref(reflect.TypeFor[viewHoldingEvent]()); err != nil {
		t.Fatal(err)
	}
	refs := map[string]string{}
	for _, kind := range []game.EventKind{"a", "b"} {
		ref, err := doc.eventRef(kindedEvent{Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		refs[string(kind)] = ref.Ref
	}
	if _, has := doc.defs["kindedEvent"].Properties.Get(game.EventKindField); has {
		t.Error("the def the view uses gained a kind")
	}
	for kind, ref := range refs {
		def := doc.defs[ref[len(defsPrefix):]]
		kindProperty, _ := def.Properties.Get(game.EventKindField)
		if kindProperty == nil || kindProperty.Const != kind || countOf(def.Required, game.EventKindField) != 1 {
			t.Errorf("event %s at %s: kind %+v, required %v", kind, ref, kindProperty, def.Required)
		}
	}
}

func countOf(values []string, value string) int {
	n := 0
	for _, v := range values {
		if v == value {
			n++
		}
	}
	return n
}
