package gametest

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func checkSecrecy[Opts, Action, View any](t testing.TB, script Script[Opts, Action], obs observation[View], where string) {
	t.Helper()
	events := eventStrings(t, obs.Events)
	views := make(map[game.Seat][]string, len(obs.Views))
	for seat, view := range obs.Views {
		views[seat] = viewStrings(t, view)
	}
	if !obs.Finished {
		checkAbsent(t, events, script.Secrets, where+": events")
		for seat, view := range views {
			checkAbsent(t, view, script.Secrets, where+": "+string(seat)+"'s view")
		}
	}
	for owner, private := range obs.Private {
		checkAbsent(t, events, private, where+": events")
		for seat, view := range views {
			if seat != owner {
				checkAbsent(t, view, private, where+": "+string(seat)+"'s view, private to "+string(owner))
			}
		}
	}
}

func checkAbsent(t testing.TB, encoded []string, forbidden []string, where string) {
	t.Helper()
	for _, s := range forbidden {
		for _, value := range encoded {
			if strings.Contains(value, s) {
				t.Errorf("%s leaks %q in %q", where, s, value)
			}
		}
	}
}

func viewStrings(t testing.TB, view any) []string {
	t.Helper()
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("encode view %T: %v", view, err)
	}
	return jsonStrings(t, raw)
}

func eventStrings(t testing.TB, events []game.Event) []string {
	t.Helper()
	var out []string
	for _, e := range events {
		raw, err := game.EncodeEvent(e)
		if err != nil {
			t.Fatalf("%v", err)
		}
		out = append(out, jsonStrings(t, raw)...)
	}
	return out
}

// jsonStrings returns every key, string and number in the JSON document raw, decoded.
func jsonStrings(t testing.TB, raw []byte) []string {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var doc any
	if err := decoder.Decode(&doc); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return appendStrings(nil, doc)
}

func appendStrings(out []string, v any) []string {
	switch v := v.(type) {
	case string:
		return append(out, v)
	case json.Number:
		return append(out, v.String())
	case []any:
		for _, item := range v {
			out = appendStrings(out, item)
		}
	case map[string]any:
		for key, item := range v {
			out = appendStrings(append(out, key), item)
		}
	}
	return out
}
