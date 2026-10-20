package gametest

import (
	"maps"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"games.nvim/server/game"
)

type Step[Action any] struct {
	Seat    game.Seat
	Action  Action
	WantErr bool
}

type Script[Opts, Action any] struct {
	Name       string
	Opts       Opts
	Challenger game.UserID
	Invitees   []game.UserID
	Seed       uint64
	Steps      []Step[Action]
	WantResult *game.Result
}

type observation[View any] struct {
	Views    map[game.Seat]View
	ToAct    []game.Seat
	Result   game.Result
	Finished bool
}

type replay[View any] struct {
	Seats       map[game.UserID]game.Seat
	Observed    []observation[View]
	FinalResult game.Result
	Finished    bool
}

func Run[Opts, Action, View any](t *testing.T, rules game.Rules[Opts, Action, View], scripts []Script[Opts, Action]) {
	t.Helper()
	for _, script := range scripts {
		t.Run(script.Name, func(t *testing.T) {
			first := play(t, rules, script)
			second := play(t, rules, script)
			if diff := cmp.Diff(first, second); diff != "" {
				t.Errorf("replay is not deterministic (-first +second):\n%s", diff)
			}
			checkWantResult(t, script, first)
			checkEliminate(t, rules, script)
		})
	}
}

func play[Opts, Action, View any](t *testing.T, rules game.Rules[Opts, Action, View], script Script[Opts, Action]) replay[View] {
	t.Helper()
	state, seats, err := rules.New(script.Opts, script.Challenger, script.Invitees, game.StepRNG(script.Seed, 0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	allSeats := sortedSeats(seats)
	observed := []observation[View]{observe(t, state, allSeats)}
	applied := uint64(0)
	for i, step := range script.Steps {
		before := observe(t, state, allSeats)
		next, _, err := state.Apply(step.Seat, step.Action, game.StepRNG(script.Seed, applied+1))
		if diff := cmp.Diff(before, observe(t, state, allSeats)); diff != "" {
			t.Fatalf("step %d: Apply mutated its receiver (-before +after):\n%s", i, diff)
		}
		if step.WantErr {
			if err == nil {
				t.Fatalf("step %d: want error, got none", i)
			}
			continue
		}
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		state = next
		applied++
		observed = append(observed, observe(t, state, allSeats))
	}
	result, finished := state.Outcome()
	return replay[View]{Seats: seats, Observed: observed, FinalResult: result, Finished: finished}
}

func observe[Action, View any](t *testing.T, state game.State[Action, View], seats []game.Seat) observation[View] {
	t.Helper()
	views := make(map[game.Seat]View, len(seats))
	for _, seat := range seats {
		views[seat] = state.View(seat)
	}
	result, finished := state.Outcome()
	toAct := state.ToAct()
	if finished != (len(toAct) == 0) {
		t.Fatalf("ToAct()=%v but Outcome finished=%t: ToAct must be empty exactly when finished", toAct, finished)
	}
	return observation[View]{Views: views, ToAct: toAct, Result: result, Finished: finished}
}

func sortedSeats(seats map[game.UserID]game.Seat) []game.Seat {
	return slices.Sorted(maps.Values(seats))
}

func checkWantResult[Opts, Action, View any](t *testing.T, script Script[Opts, Action], got replay[View]) {
	t.Helper()
	if script.WantResult == nil {
		return
	}
	if !got.Finished {
		t.Fatalf("want result %+v, but game is not finished", *script.WantResult)
	}
	if diff := cmp.Diff(*script.WantResult, got.FinalResult); diff != "" {
		t.Errorf("final result (-want +got):\n%s", diff)
	}
}

func checkEliminate[Opts, Action, View any](t *testing.T, rules game.Rules[Opts, Action, View], script Script[Opts, Action]) {
	t.Helper()
	state, seats, err := rules.New(script.Opts, script.Challenger, script.Invitees, game.StepRNG(script.Seed, 0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	allSeats := sortedSeats(seats)
	twoPlayer := rules.Info().Players.Max == 2
	for _, seat := range allSeats {
		before := observe(t, state, allSeats)
		eliminated, _ := state.Eliminate(seat, game.ReasonResign)
		if diff := cmp.Diff(before, observe(t, state, allSeats)); diff != "" {
			t.Fatalf("Eliminate(%s) mutated its receiver (-before +after):\n%s", seat, diff)
		}
		after := observe(t, eliminated, allSeats)
		if twoPlayer && !after.Finished {
			t.Errorf("Eliminate(%s) in a two-player game must finish it", seat)
		}
	}
}
