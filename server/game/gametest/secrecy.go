package gametest

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func checkSecrecy[Opts, Action, View any](t testing.TB, script Script[Opts, Action], obs observation[View], where string) {
	t.Helper()
	events := eventsShown(t, obs.Events)
	views := make(map[game.Seat]shown, len(obs.Views))
	for seat, view := range obs.Views {
		views[seat] = viewShown(t, view)
	}
	if !obs.Finished {
		checkSecrets(t, events, script.Secrets, where+": events")
		for seat, view := range views {
			checkSecrets(t, view, script.Secrets, where+": "+string(seat)+"'s view")
		}
	}
	for owner, private := range obs.Private {
		checkAbsent(t, events.values, private, where+": events")
		for seat, view := range views {
			if seat != owner {
				checkAbsent(t, view.values, private, where+": "+string(seat)+"'s view, private to "+string(owner))
			}
		}
	}
}

// shown is what a JSON document shows: every key, string and number in it,
// decoded, and the runs of its arrays.
type shown struct {
	values []string
	// runs are each array's strings and numbers, nested arrays' too, joined,
	// as a secret sent piece by piece reads.
	runs []string
}

// minSpannedSecretLen is the shortest secret looked for inside an array's
// run; a shorter one must be the whole run, since pieces of unrelated values
// joined spell short strings by chance.
const minSpannedSecretLen = 8

func checkSecrets(t testing.TB, doc shown, secrets []string, where string) {
	t.Helper()
	checkAbsent(t, doc.values, secrets, where)
	for _, secret := range secrets {
		if slices.ContainsFunc(doc.runs, func(run string) bool { return runLeaks(run, secret) }) {
			t.Errorf("%s leaks %q as an array", where, secret)
		}
	}
}

func runLeaks(run, secret string) bool {
	if len(secret) < minSpannedSecretLen {
		return run == secret
	}
	return strings.Contains(run, secret)
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

func viewShown(t testing.TB, view any) shown {
	t.Helper()
	return jsonShown(t, []byte(ViewJSON(t, view)))
}

func eventsShown(t testing.TB, events []game.Event) shown {
	t.Helper()
	var out shown
	for _, e := range events {
		raw, err := game.EncodeEvent(e)
		if err != nil {
			t.Fatalf("%v", err)
		}
		doc := jsonShown(t, raw)
		out.values = append(out.values, doc.values...)
		out.runs = append(out.runs, doc.runs...)
	}
	return out
}

func jsonShown(t testing.TB, raw []byte) shown {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var doc any
	if err := decoder.Decode(&doc); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return shown{values: appendStrings(nil, doc), runs: appendRuns(nil, doc)}
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

func appendRuns(out []string, v any) []string {
	switch v := v.(type) {
	case []any:
		var run strings.Builder
		writeRun(&run, v)
		out = append(out, run.String())
		for _, item := range v {
			out = appendRuns(out, item)
		}
	case map[string]any:
		for _, item := range v {
			out = appendRuns(out, item)
		}
	}
	return out
}

func writeRun(run *strings.Builder, items []any) {
	for _, item := range items {
		switch item := item.(type) {
		case string:
			run.WriteString(item)
		case json.Number:
			run.WriteString(item.String())
		case []any:
			writeRun(run, item)
		}
	}
}
