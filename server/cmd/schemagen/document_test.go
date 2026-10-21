package main

import (
	"reflect"
	"slices"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
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
	doc := newDocument(nil, nil)
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

type Cell int

type viewWithTwoCells struct {
	Board Cell      `json:"board"`
	Last  grid.Cell `json:"last"`
}

func TestTwoTypesOfOneNameFail(t *testing.T) {
	if _, err := newDocument(nil, nil).ref(reflect.TypeFor[viewWithTwoCells]()); err == nil {
		t.Error("grid.Cell and Cell share a def")
	}
}

type kindWithoutValues string

type kindWithValues string

func (kindWithValues) EnumValues() []string { return []string{"a", "b"} }

func TestAGameKindNeedsEnumValues(t *testing.T) {
	for _, tc := range []struct {
		kind    reflect.Type
		wantErr bool
	}{
		{reflect.TypeFor[kindWithoutValues](), true},
		{reflect.TypeFor[kindWithValues](), false},
		{reflect.TypeFor[game.Seat](), false},
	} {
		holder := reflect.StructOf([]reflect.StructField{{Name: "Field", Type: tc.kind, Tag: `json:"field"`}})
		doc := newDocument(nil, nil)
		if _, err := doc.ref(holder); (err != nil) != tc.wantErr {
			t.Errorf("%s: got %v, want error %t", tc.kind, err, tc.wantErr)
		}
		if e, ok := doc.enums[tc.kind]; ok && !slices.Equal(e.values, []string{"a", "b"}) {
			t.Errorf("%s: enum %v", tc.kind, e.values)
		}
	}
}
