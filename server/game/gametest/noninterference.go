package gametest

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// Hidden is how NonInterference changes a game's hidden information.
type Hidden[Action, View any] struct {
	// Perturb is s with information changed that only the seats it returns
	// may see, or no seats when s holds nothing of owner's to change. A
	// game.Opener fresh from New stays one, its opening events those New
	// would have sent for the change.
	Perturb func(s game.State[Action, View], owner game.Seat) (game.State[Action, View], []game.Seat)
	// Probes, if set, are actions for the seat to act, read from its view,
	// whose errors must not depend on the change either.
	Probes func(View) []Action
	// Reveals, if set, are the seats step may tell what was changed of
	// owners, such as the challenger of a hand; with any, the step's events
	// and outcome may differ, and those seats' views after it.
	Reveals func(s game.State[Action, View], step Step[Action], owners []game.Seat) []game.Seat
}

// NonInterference plays script and, before each step of the unfinished game,
// changes every seat's hidden information in turn: what any other seat is
// sent must stay byte for byte the same. That is its view, the seats to act
// and the outcome, the opening events, and the step's events, outcome and
// views after it unless Reveals says the step may tell. The step's error
// must stay the same too, unless the step is the owners' own, whose action
// the change may make illegal, or the actor may be told. It fails when no
// view or no step was compared.
func NonInterference[Opts, Action, View any](t testing.TB, rules game.Rules[Opts, Action, View], script Script[Opts, Action], hidden Hidden[Action, View]) {
	t.Helper()
	state, seats, err := start(rules, script)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	allSeats := sortedSeats(seats)
	viewsCompared, stepsCompared := 0, 0
	applied := game.Seq(0)
	for i, step := range script.Steps {
		if _, finished := state.Outcome(); finished {
			break
		}
		rng := func() *rand.Rand { return game.StepRNG(script.Seed, applied+1) }
		for _, owner := range allSeats {
			changed, owners := hidden.Perturb(state, owner)
			if len(owners) == 0 {
				continue
			}
			where := fmt.Sprintf("step %d, with the hidden information of %v changed", i, owners)
			watchers := without(allSeats, owners)
			if applied == 0 {
				requireSameOpening(t, state, changed, where)
			}
			viewsCompared += requireSameViews(t, state, changed, watchers, where)
			requireSamePublic(t, state, changed, where)
			var told []game.Seat
			if hidden.Reveals != nil {
				told = hidden.Reveals(state, step, owners)
			}
			errorMayDiffer := step.Kind == StepAction && (slices.Contains(owners, step.Seat) || slices.Contains(told, step.Seat))
			if !errorMayDiffer {
				requireSameProbeErrors(t, state, changed, step, hidden.Probes, rng, where)
			}
			if requireSameStep(t, state, changed, step, rng, sameStep{watchers: without(watchers, told), errorMayDiffer: errorMayDiffer, told: len(told) > 0}, where) {
				stepsCompared++
			}
		}
		next, _, err := advance(t, state, step, rng())
		switch {
		case err != nil && !step.WantErr:
			t.Fatalf("step %d: %v", i, err)
		case err == nil && step.WantErr:
			t.Fatalf("step %d: want error, got none", i)
		case err == nil:
			state = next
			applied++
		}
	}
	if viewsCompared == 0 {
		t.Fatalf("no view was compared: Perturb changed nothing, or left no other seat to watch")
	}
	if stepsCompared == 0 {
		t.Fatalf("no step was compared: every step after a change failed or told every seat")
	}
}

func without(seats, gone []game.Seat) []game.Seat {
	return slices.DeleteFunc(slices.Clone(seats), func(s game.Seat) bool { return slices.Contains(gone, s) })
}

func requireSameOpening[Action, View any](t testing.TB, state, changed game.State[Action, View], where string) {
	t.Helper()
	opener, ok := state.(game.Opener)
	if !ok {
		return
	}
	changedOpener, ok := changed.(game.Opener)
	if !ok {
		t.Fatalf("%s: Perturb turned the %T fresh from New, a game.Opener, into a %T, which is not", where, state, changed)
	}
	if diff := cmp.Diff(eventJSON(t, opener.OpeningEvents()), eventJSON(t, changedOpener.OpeningEvents())); diff != "" {
		t.Fatalf("%s: the opening events depend on it (-before +changed):\n%s", where, diff)
	}
}

// requireSameViews compares the views of watchers and returns how many it compared.
func requireSameViews[Action, View any](t testing.TB, state, changed game.State[Action, View], watchers []game.Seat, where string) int {
	t.Helper()
	for _, seat := range watchers {
		if diff := cmp.Diff(ViewJSON(t, state.View(seat)), ViewJSON(t, changed.View(seat))); diff != "" {
			t.Fatalf("%s: %s's view depends on it (-before +changed):\n%s", where, seat, diff)
		}
	}
	return len(watchers)
}

// requireSamePublic compares what every seat is sent besides the views: the
// seats to act and knocked out, the outcome and the next tick.
func requireSamePublic[Action, View any](t testing.TB, state, changed game.State[Action, View], where string) {
	t.Helper()
	if diff := cmp.Diff(observe(t, state, nil), observe(t, changed, nil)); diff != "" {
		t.Fatalf("%s: the seats to act or the outcome depend on it (-before +changed):\n%s", where, diff)
	}
}

func requireSameProbeErrors[Action, View any](t testing.TB, state, changed game.State[Action, View], step Step[Action], probes func(View) []Action, rng func() *rand.Rand, where string) {
	t.Helper()
	if probes == nil || step.Kind != StepAction {
		return
	}
	for _, action := range probes(state.View(step.Seat)) {
		probe := Step[Action]{Seat: step.Seat, Action: action}
		_, _, err := advance(t, state, probe, rng())
		_, _, changedErr := advance(t, changed, probe, rng())
		if fmt.Sprint(err) != fmt.Sprint(changedErr) {
			t.Fatalf("%s: the error of %+v depends on it: %v / %v", where, action, err, changedErr)
		}
	}
}

// sameStep is what requireSameStep may let differ.
type sameStep struct {
	watchers       []game.Seat
	errorMayDiffer bool
	told           bool
}

// requireSameStep makes step from state and from changed, each with a fresh
// rng, and reports whether it compared anything: their errors, and once both
// accept it, their events and outcome unless some seat is told, and the
// views of the watchers after it.
func requireSameStep[Action, View any](t testing.TB, state, changed game.State[Action, View], step Step[Action], rng func() *rand.Rand, same sameStep, where string) bool {
	t.Helper()
	next, events, err := advance(t, state, step, rng())
	changedNext, changedEvents, changedErr := advance(t, changed, step, rng())
	if !same.errorMayDiffer && fmt.Sprint(err) != fmt.Sprint(changedErr) {
		t.Fatalf("%s: the step's error depends on it: %v / %v", where, err, changedErr)
	}
	if err != nil || changedErr != nil {
		return !same.errorMayDiffer
	}
	if !same.told {
		if diff := cmp.Diff(eventJSON(t, events), eventJSON(t, changedEvents)); diff != "" {
			t.Fatalf("%s: the step's events depend on it (-before +changed):\n%s", where, diff)
		}
		requireSamePublic(t, next, changedNext, where+", after the step")
	}
	viewed := requireSameViews(t, next, changedNext, same.watchers, where+", after the step")
	return !same.told || viewed > 0
}
