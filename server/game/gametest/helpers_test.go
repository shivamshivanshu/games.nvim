package gametest

import (
	"slices"
	"strings"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

func TestPlayToFinishPlaysAScriptRunAccepts(t *testing.T) {
	addFive := func(CounterView, game.Seat) CounterAction { return adding(5) }
	users := Users(2)
	steps := PlayToFinish(t, CounterRules{}, CounterOpts{Step: "1"}, users, 1, addFive, CounterTarget)
	won := game.WinFor(ReasonTargetReached, "b")
	Run(t, CounterRules{}, []Script[CounterOpts, CounterAction]{{
		Name: "played to the finish", Opts: CounterOpts{Step: "1"}, Challenger: users[0], Invitees: users[1:], Seed: 1, Steps: steps, WantResult: &won,
	}})
	if len(steps) != 2 {
		t.Errorf("played %d steps, want 2", len(steps))
	}
}

func TestReplayGivesEveryStateToTheFinish(t *testing.T) {
	addFive := func(CounterView, game.Seat) CounterAction { return adding(5) }
	opts := CounterOpts{Step: "1"}
	users := Users(2)
	start, _ := newCounter(opts, users[0], users[1:])
	states := Replay(t, CounterRules{}.Info(), start, PlayToFinish(t, CounterRules{}, opts, users, 1, addFive, CounterTarget), 1)
	totals := xslices.Map(states, func(s counterState) int { return s.View("a").Total })
	if !slices.Equal(totals, []int{0, 5, 10}) {
		t.Errorf("totals %v, want [0 5 10]", totals)
	}
	if _, finished := states[len(states)-1].Outcome(); !finished {
		t.Error("the last state is not finished")
	}
}

// unwrappedCounter's Apply is its counter's, so the next state is a counterState.
type unwrappedCounter struct{ counterState }

func TestPlayToFinishAndMustApplyFail(t *testing.T) {
	addOne := func(CounterView, game.Seat) CounterAction { return adding(1) }
	start, _ := newCounter(CounterOpts{}, "u", []game.UserID{"v"})
	tests := []struct {
		name  string
		check func(testing.TB)
		want  string
	}{
		{"no finish within the steps", func(tb testing.TB) {
			PlayToFinish(tb, CounterRules{}, CounterOpts{Step: "1"}, Users(2), 1, addOne, 3)
		}, "no finish within 3 steps"},
		{"a seat not to act", func(tb testing.TB) { MustApply(tb, CounterRules{}.Info(), start, "b", adding(1), nil) }, "not your turn"},
		{"an action the rules refuse", func(tb testing.TB) { MustApply(tb, CounterRules{}.Info(), start, "a", adding(-1), nil) }, "negative add"},
		{"an action its catalogue refuses", func(tb testing.TB) {
			addIsOthers := game.Info{Actions: []game.ActionSpec{{Kind: counterAdd}, {Kind: "other", Input: &game.ActionInput{Field: "Add"}}}}
			MustApply(tb, addIsOthers, start, "a", adding(1), nil)
		}, `add takes no field "Add", which is other's`},
		{"a next state of another type", func(tb testing.TB) {
			MustApply(tb, CounterRules{}.Info(), unwrappedCounter{start}, "a", adding(1), nil)
		}, "the next state is a gametest.counterState, not a gametest.unwrappedCounter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failures := recordFailures(tt.check)
			if !slices.ContainsFunc(failures, func(f string) bool { return strings.Contains(f, tt.want) }) {
				t.Errorf("want a failure containing %q, got %q", tt.want, failures)
			}
		})
	}
}
