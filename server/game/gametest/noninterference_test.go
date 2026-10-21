package gametest

import (
	"slices"
	"strings"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type perturbation = func(game.State[tableAction, tableView], game.Seat) (game.State[tableAction, tableView], []game.Seat)

type revelation = func(game.State[tableAction, tableView], Step[tableAction], []game.Seat) []game.Seat

// revealsShowdown tells every seat the hand of an owner who shows it.
func revealsShowdown(st game.State[tableAction, tableView], step Step[tableAction], owners []game.Seat) []game.Seat {
	if slices.Contains(owners, step.Seat) {
		return st.(tableState).seats
	}
	return nil
}

func TestNonInterference(t *testing.T) {
	untouched := func(st game.State[tableAction, tableView], _ game.Seat) (game.State[tableAction, tableView], []game.Seat) {
		return st, nil
	}
	everyOwner := func(st game.State[tableAction, tableView], owner game.Seat) (game.State[tableAction, tableView], []game.Seat) {
		changed, _ := cutHand(st, owner)
		return changed, st.(tableState).seats
	}
	alwaysTells := func(st game.State[tableAction, tableView], _ Step[tableAction], _ []game.Seat) []game.Seat {
		return st.(tableState).seats
	}
	tests := []struct {
		name    string
		flaw    flaw
		perturb perturbation
		reveals revelation
		want    string
	}{
		{"a hand reaches no other seat", honest, cutHand, revealsShowdown, ""},
		{"a hand in another seat's view", leakPrivateInView, cutHand, revealsShowdown, "step 0, with the hidden information of [s1] changed: s2's view depends on it"},
		{"a hand in another seat's event", leakNeighbourInEvent, cutHand, revealsShowdown, "step 0, with the hidden information of [s2] changed: the step's events depend on it"},
		{"a hand in the opening events", leakNeighbourInOpening, cutHand, revealsShowdown, "step 0, with the hidden information of [s2] changed: the opening events depend on it"},
		{"a hand that decides the outcome", finishOnNeighbourCut, cutHand, revealsShowdown, "step 0, with the hidden information of [s2] changed, after the step: the seats to act or the outcome depend on it"},
		{"an owner's step that shows its hand untold", honest, cutHand, nil, "step 0, with the hidden information of [s1] changed: the step's events depend on it"},
		{"nothing perturbed", honest, untouched, revealsShowdown, "no view was compared"},
		{"no seat left to watch", leakPrivateInView, everyOwner, revealsShowdown, "no view was compared"},
		{"every step tells every seat", leakNeighbourInEvent, cutHand, alwaysTells, "no step was compared"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hidden := Hidden[tableAction, tableView]{Perturb: tt.perturb, Reveals: tt.reveals}
			failures := recordFailures(func(tb testing.TB) { NonInterference(tb, tableRules{max: 3, flaw: tt.flaw}, tableScript(3), hidden) })
			checkFailure(t, failures, tt.want)
		})
	}
}

func TestNonInterferenceRequiresTheScriptsErrors(t *testing.T) {
	script := tableScript(3)
	script.Steps[0].WantErr = true
	failures := recordFailures(func(tb testing.TB) {
		NonInterference(tb, tableRules{max: 3}, script, Hidden[tableAction, tableView]{Perturb: cutHand, Reveals: revealsShowdown})
	})
	checkFailure(t, failures, "step 0: want error, got none")
}

func checkFailure(t *testing.T, failures []string, want string) {
	t.Helper()
	switch {
	case want == "" && len(failures) > 0:
		t.Errorf("want no failure, got %q", failures)
	case want != "" && !slices.ContainsFunc(failures, func(f string) bool { return strings.HasPrefix(f, want) }):
		t.Errorf("want a failure starting %q, got %q", want, failures)
	}
}
