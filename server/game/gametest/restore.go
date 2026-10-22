package gametest

import (
	"encoding/json"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// wireObservation is what the core sends of a state reached by a step.
type wireObservation struct {
	Views      map[game.Seat]string
	Events     []string
	ToAct      []game.Seat
	Result     game.Result
	Finished   bool
	KnockedOut []game.Seat
	NextTick   time.Duration
	TickDue    bool
}

// checkRestoredPlay replays the script through game.Adapt the way the core
// restores a game: from a fresh New, the accepted steps only, each action as
// its JSON. Each state reached must send what play did there. A rejected
// action that changed the state it was refused by, or an action field the
// JSON loses, shows up as a difference at the step that lost it.
//
// That replay is viewed at every step, as play is, so a View or Conclude that
// changes hidden state would change both alike. The core restores blind, so
// the check restores blind too, at the restorePoints, and compares only there.
func checkRestoredPlay[Opts, Action, View any](t testing.TB, rules game.Rules[Opts, Action, View], script Script[Opts, Action], played replay[View]) {
	t.Helper()
	persisted := slices.DeleteFunc(slices.Clone(script.Steps), func(s Step[Action]) bool { return s.WantErr })
	state, allSeats := newAdapted(t, rules, script, played)
	checkRestored(t, played.Observed[0], observeWire(t, state, allSeats, state.OpeningEvents()), "replayed fresh from New")
	for i, step := range persisted {
		var events []json.RawMessage
		state, events = advanceWire(t, state, step, script.Seed, game.Seq(i+1))
		checkRestored(t, played.Observed[i+1], observeWire(t, state, allSeats, events), "replayed to persisted step "+strconv.Itoa(i+1))
	}
	for _, k := range restorePoints(len(persisted)) {
		state, _ := newAdapted(t, rules, script, played)
		events := state.OpeningEvents()
		for i, step := range persisted[:k] {
			state, events = advanceWire(t, state, step, script.Seed, game.Seq(i+1))
		}
		checkRestored(t, played.Observed[k], observeWire(t, state, allSeats, events), "restored blind to persisted step "+strconv.Itoa(k))
	}
}

// restorePoints are where of n persisted steps to restore blind: each of the
// first few, then every restoreStride-th and the last, so that a long
// script's restores replay a fraction of what restoring at every step would.
func restorePoints(n int) []int {
	var points []int
	for k := 0; k <= n; k++ {
		if k <= restoreEvery || k%restoreStride == 0 || k == n {
			points = append(points, k)
		}
	}
	return points
}

const (
	restoreEvery  = 8
	restoreStride = 16
)

func newAdapted[Opts, Action, View any](t testing.TB, rules game.Rules[Opts, Action, View], script Script[Opts, Action], played replay[View]) (game.ErasedState, []game.Seat) {
	t.Helper()
	challenger, invitees := script.players()
	state, seats, err := game.Adapt(rules).New(optionsOf(t, script.Opts), challenger, invitees, game.StepRNG(script.Seed, 0))
	if err != nil {
		t.Fatalf("New through game.Adapt: %v", err)
	}
	if diff := cmp.Diff(played.Seats, seats); diff != "" {
		t.Fatalf("New through game.Adapt seats differ (-play +restored):\n%s", diff)
	}
	return state, sortedSeats(seats)
}

func checkRestored[View any](t testing.TB, played observation[View], restored wireObservation, where string) {
	t.Helper()
	if diff := cmp.Diff(onTheWire(t, played), restored); diff != "" {
		t.Errorf("%s: a restored game sends otherwise than play did (-play +restored):\n%s", where, diff)
	}
}

func advanceWire[Action any](t testing.TB, state game.ErasedState, step Step[Action], seed game.Seed, seq game.Seq) (game.ErasedState, []json.RawMessage) {
	t.Helper()
	rng := game.StepRNG(seed, seq)
	var next game.ErasedState
	var events []json.RawMessage
	var err error
	switch step.Kind {
	case StepTick:
		next, events, err = state.Tick(rng)
	case StepPause:
		next = state.Pause()
	default:
		next, events, err = state.Apply(step.Seat, actionJSON(t, step.Action), rng)
	}
	if err != nil {
		t.Fatalf("persisted step %d through game.Adapt: %v", seq, err)
	}
	return next, events
}

// optionsOf is opts as the core holds them: its JSON fields, by name.
func optionsOf[Opts any](t testing.TB, opts Opts) game.Options {
	t.Helper()
	raw, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("encode options %+v: %v", opts, err)
	}
	var options game.Options
	if err := json.Unmarshal(raw, &options); err != nil {
		t.Fatalf("options %s are not an object of strings: %v", raw, err)
	}
	return options
}

func observeWire(t testing.TB, state game.ErasedState, seats []game.Seat, events []json.RawMessage) wireObservation {
	t.Helper()
	views := make(map[game.Seat]string, len(seats))
	for _, seat := range seats {
		raw, err := state.View(seat)
		if err != nil {
			t.Fatalf("view of %s: %v", seat, err)
		}
		views[seat] = string(raw)
	}
	encoded := make([]string, len(events))
	for i, e := range events {
		encoded[i] = string(e)
	}
	result, finished := state.Outcome()
	nextTick, tickDue := state.NextTick()
	return wireObservation{Views: views, Events: encoded, ToAct: state.ToAct(), Result: result, Finished: finished, KnockedOut: state.KnockedOut(), NextTick: nextTick, TickDue: tickDue}
}

func onTheWire[View any](t testing.TB, obs observation[View]) wireObservation {
	t.Helper()
	views := make(map[game.Seat]string, len(obs.Views))
	for seat, view := range obs.Views {
		views[seat] = ViewJSON(t, view)
	}
	return wireObservation{Views: views, Events: eventJSON(t, obs.Events), ToAct: obs.ToAct, Result: obs.Result, Finished: obs.Finished, KnockedOut: obs.KnockedOut, NextTick: obs.NextTick, TickDue: obs.TickDue}
}

func eventJSON(t testing.TB, events []game.Event) []string {
	t.Helper()
	out := make([]string, len(events))
	for i, e := range events {
		raw, err := game.EncodeEvent(e)
		if err != nil {
			t.Fatalf("%v", err)
		}
		out[i] = string(raw)
	}
	return out
}
