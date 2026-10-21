package gametest

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func TestRunCounterRules(t *testing.T) {
	Run(t, CounterRules{}, counterScripts)
	Run(t, ScoredCounterRules{}, counterScripts)
}

func adding(n int) CounterAction { return CounterAction{Kind: counterAdd, Add: n} }

var counterScripts = []Script[CounterOpts, CounterAction]{
	{
		Name:       "accepted and rejected steps",
		Opts:       CounterOpts{Step: "2"},
		Challenger: "u",
		Seed:       1,
		Steps: []Step[CounterAction]{
			{Seat: "a", Action: adding(3)},
			{Seat: "a", Action: adding(-1), WantErr: true},
			{Seat: "a", Action: adding(1)},
		},
	},
	{
		Name:       "two players alternate until one reaches the target",
		Challenger: "u",
		Invitees:   []game.UserID{"v"},
		Seed:       1,
		Steps: []Step[CounterAction]{
			{Seat: "b", Action: adding(1), WantErr: true},
			{Seat: "a", Action: adding(5)},
			{Seat: "b", Action: adding(5)},
			{Seat: "a", Action: adding(1), WantErr: true},
		},
		WantResult: &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{"b"}, Reason: ReasonTargetReached},
	},
}

func TestRunMetronomeRules(t *testing.T) {
	tick, pause := Step[MetronomeAction]{Kind: StepTick}, Step[MetronomeAction]{Kind: StepPause}
	act := func(kind game.ActionKind, wantErr bool) Step[MetronomeAction] {
		return Step[MetronomeAction]{Seat: MetronomeSeat, Action: MetronomeAction{Kind: kind}, WantErr: wantErr}
	}
	won := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{MetronomeSeat}, Reason: ReasonTargetReached}
	Run(t, MetronomeRules{}, []Script[struct{}, MetronomeAction]{
		{
			Name:       "a core pause holds the ticks until the player resumes",
			Challenger: "u",
			Seed:       1,
			Steps:      []Step[MetronomeAction]{tick, pause, act(MetronomePause, true), act(MetronomeResume, false), tick},
		},
		{Name: "ticks to the target", Challenger: "u", Seed: 2, Steps: ticksToTarget(2), WantResult: &won},
	})
}

// ticksToTarget are the tick steps that take a metronome to its target from seed's start.
func ticksToTarget(seed game.Seed) []Step[MetronomeAction] {
	var steps []Step[MetronomeAction]
	var s game.State[MetronomeAction, MetronomeView] = metronomeState{}
	for _, done := s.Outcome(); !done; _, done = s.Outcome() {
		steps = append(steps, Step[MetronomeAction]{Kind: StepTick})
		s, _ = s.(metronomeState).Tick(game.StepRNG(seed, game.Seq(len(steps))))
	}
	return steps
}

// endlessMetronome wants ticks even once finished.
type endlessMetronome struct{ metronomeState }

func (s endlessMetronome) NextTick() (time.Duration, bool) { return MetronomeInterval, true }

// restlessMetronome ticks on through a pause.
type restlessMetronome struct{ metronomeState }

func (s restlessMetronome) Pause() game.State[MetronomeAction, MetronomeView] { return s }

func TestRunRejectsTicksTheGameDoesNotWant(t *testing.T) {
	finished := metronomeState{Ending: game.Ending{}.Concluded(game.Result{Kind: game.ResultDraw})}
	tests := []struct {
		name  string
		check func(testing.TB)
		want  string
	}{
		{"a finished state that wants a tick", func(tb testing.TB) {
			observe(tb, game.State[MetronomeAction, MetronomeView](endlessMetronome{finished}), []game.Seat{MetronomeSeat})
		}, "a finished state wants a tick"},
		{"a pause that leaves a tick due", func(tb testing.TB) {
			checkStops(tb, game.State[MetronomeAction, MetronomeView](restlessMetronome{}), []game.Seat{MetronomeSeat}, "fresh")
		}, "fresh: Pause left a tick due"},
		{"a tick of a paused state", func(tb testing.TB) {
			_, _, _ = advance(tb, game.State[MetronomeAction, MetronomeView](metronomeState{paused: true}), Step[MetronomeAction]{Kind: StepTick}, nil)
		}, "a tick step on a state that wants no tick"},
		{"a tick of a state that does not tick", func(tb testing.TB) {
			conform(tb, CounterRules{}, Script[CounterOpts, CounterAction]{Challenger: "u", Steps: []Step[CounterAction]{{Kind: StepTick}}})
		}, "which is not a game.Ticking"},
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
