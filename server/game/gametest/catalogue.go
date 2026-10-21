package gametest

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// checkActionsCatalogued requires the kind of every action the script makes
// to be in info's Actions; a rejected action may be of any kind.
func checkActionsCatalogued[Opts, Action any](t testing.TB, info game.Info, script Script[Opts, Action]) {
	t.Helper()
	for i, step := range script.Steps {
		if step.Kind != StepAction || step.WantErr {
			continue
		}
		kind := actionKind(t, step.Action)
		if !slices.ContainsFunc(info.Actions, func(spec game.ActionSpec) bool { return spec.Kind == kind }) {
			t.Errorf("step %d: action kind %q is not in Info.Actions", i, kind)
		}
	}
}

func actionKind(t testing.TB, action any) game.ActionKind {
	t.Helper()
	var envelope struct {
		Kind game.ActionKind `json:"kind"`
	}
	if err := json.Unmarshal(actionJSON(t, action), &envelope); err != nil {
		t.Fatalf("action %+v is not a JSON object: %v", action, err)
	}
	return envelope.Kind
}

func actionJSON(t testing.TB, action any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(action)
	if err != nil {
		t.Fatalf("encode action %+v: %v", action, err)
	}
	return raw
}

func checkEventsCatalogued(t testing.TB, info game.Info, events []game.Event, where string) {
	t.Helper()
	for i, e := range events {
		if !slices.ContainsFunc(info.Events, func(c game.Event) bool { return c.EventKind() == e.EventKind() }) {
			t.Errorf("%s: event %d of kind %q is not in Info.Events", where, i, e.EventKind())
		}
	}
}
