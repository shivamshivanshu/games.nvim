package gametest

import (
	"maps"
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
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
	Seed       game.Seed
	Steps      []Step[Action]
	WantResult *game.Result
	// PrivateOf names, per state, the strings only that seat may see: never in another seat's view, nor in any event.
	PrivateOf Privacy
	// Secrets must not appear in any view or event until the game finishes.
	Secrets []string
}

type observation[View any] struct {
	Views    map[game.Seat]View
	Events   []game.Event
	Private  map[game.Seat][]string
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
		t.Run(script.Name, func(t *testing.T) { conform(t, rules, script) })
	}
}

func conform[Opts, Action, View any](t testing.TB, rules game.Rules[Opts, Action, View], script Script[Opts, Action]) {
	t.Helper()
	first, final := play(t, rules, script)
	second, _ := play(t, rules, script)
	if diff := cmp.Diff(first, second); diff != "" {
		t.Errorf("replay is not deterministic (-first +second):\n%s", diff)
	}
	for i, obs := range first.Observed {
		checkSecrecy(t, script, obs, "observation "+strconv.Itoa(i))
	}
	checkWantResult(t, script, first)
	if first.Finished {
		checkEliminateFinishedIsNoOp(t, script.PrivateOf, final, sortedSeats(first.Seats), "after the script")
	}
	checkEliminate(t, rules, script)
}

func play[Opts, Action, View any](t testing.TB, rules game.Rules[Opts, Action, View], script Script[Opts, Action]) (replay[View], game.State[Action, View]) {
	t.Helper()
	state, seats, err := rules.New(script.Opts, script.Challenger, script.Invitees, game.StepRNG(script.Seed, 0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	allSeats := sortedSeats(seats)
	var opening []game.Event
	if opener, ok := state.(game.Opener); ok {
		opening = opener.OpeningEvents()
	}
	observed := []observation[View]{record(t, script.PrivateOf, state, allSeats, opening)}
	applied := game.Seq(0)
	for i, step := range script.Steps {
		before := observe(t, state, allSeats)
		next, events, err := state.Apply(step.Seat, step.Action, game.StepRNG(script.Seed, applied+1))
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
		observed = append(observed, record(t, script.PrivateOf, state, allSeats, events))
	}
	result, finished := state.Outcome()
	return replay[View]{Seats: seats, Observed: observed, FinalResult: result, Finished: finished}, state
}

// record observes state as reached by a step that emitted events.
func record[Action, View any](t testing.TB, privacy Privacy, state game.State[Action, View], seats []game.Seat, events []game.Event) observation[View] {
	t.Helper()
	obs := observe(t, state, seats)
	obs.Events = events
	if privacy == nil {
		return obs
	}
	obs.Private = make(map[game.Seat][]string, len(seats))
	for _, seat := range seats {
		private, err := privacy.privateTo(state, seat)
		if err != nil {
			t.Fatalf("%v", err)
		}
		obs.Private[seat] = private
	}
	return obs
}

func observe[Action, View any](t testing.TB, state game.State[Action, View], seats []game.Seat) observation[View] {
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

func checkWantResult[Opts, Action, View any](t testing.TB, script Script[Opts, Action], got replay[View]) {
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

func checkEliminate[Opts, Action, View any](t testing.TB, rules game.Rules[Opts, Action, View], script Script[Opts, Action]) {
	t.Helper()
	state, seats, err := rules.New(script.Opts, script.Challenger, script.Invitees, game.StepRNG(script.Seed, 0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	allSeats := sortedSeats(seats)
	solo := rules.Info().Players.Max == 1
	for _, seat := range allSeats {
		_, after := eliminate(t, script.PrivateOf, state, seat, allSeats)
		checkSecrecy(t, script, after, "eliminating only "+string(seat))
		checkEliminationOutcome(t, solo, len(allSeats)-1, after, seat)
		checkEliminatedNotToAct(t, after, []game.Seat{seat})
	}
	for i, seat := range allSeats {
		var after observation[View]
		state, after = eliminate(t, script.PrivateOf, state, seat, allSeats)
		checkSecrecy(t, script, after, "eliminating "+string(seat))
		checkEliminationOutcome(t, solo, len(allSeats)-1-i, after, seat)
		checkEliminatedNotToAct(t, after, allSeats[:i+1])
		if after.Finished {
			checkEliminateFinishedIsNoOp(t, script.PrivateOf, state, allSeats, "after eliminating "+string(seat))
			return
		}
	}
}

func eliminate[Action, View any](t testing.TB, privacy Privacy, state game.State[Action, View], seat game.Seat, allSeats []game.Seat) (game.State[Action, View], observation[View]) {
	t.Helper()
	before := observe(t, state, allSeats)
	next, events := state.Eliminate(seat, game.ReasonResign)
	if diff := cmp.Diff(before, observe(t, state, allSeats)); diff != "" {
		t.Fatalf("Eliminate(%s) mutated its receiver (-before +after):\n%s", seat, diff)
	}
	return next, record(t, privacy, next, allSeats, events)
}

func checkEliminationOutcome[View any](t testing.TB, solo bool, remaining int, after observation[View], seat game.Seat) {
	t.Helper()
	switch {
	case remaining >= 2 && after.Finished:
		t.Errorf("Eliminate(%s) finished the game with %d seats remaining; play must continue while 2 or more remain", seat, remaining)
	case remaining == 1 && !after.Finished:
		t.Errorf("Eliminate(%s) left one seat but did not finish the game", seat)
	case remaining == 0 && solo && !after.Finished:
		t.Errorf("Eliminate(%s) of the only seat must finish a solo game", seat)
	}
}

func checkEliminatedNotToAct[View any](t testing.TB, after observation[View], eliminated []game.Seat) {
	t.Helper()
	for _, seat := range eliminated {
		if slices.Contains(after.ToAct, seat) {
			t.Errorf("eliminated %s is still in ToAct %v; an eliminated seat must not act", seat, after.ToAct)
		}
	}
}

func checkEliminateFinishedIsNoOp[Action, View any](t testing.TB, privacy Privacy, finished game.State[Action, View], allSeats []game.Seat, when string) {
	t.Helper()
	before := record(t, privacy, finished, allSeats, nil)
	for _, seat := range allSeats {
		_, after := eliminate(t, privacy, finished, seat, allSeats)
		if len(after.Events) > 0 {
			t.Errorf("%s: Eliminate(%s) on a finished game emitted events %v; it must be a no-op", when, seat, after.Events)
		}
		after.Events = nil
		if diff := cmp.Diff(before, after); diff != "" {
			t.Errorf("%s: Eliminate(%s) on a finished game must be a no-op (-before +after):\n%s", when, seat, diff)
		}
	}
}
