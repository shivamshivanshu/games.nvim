package gametest

import (
	"encoding/json"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

var userNames = []game.UserID{"alice", "bob", "carol", "dave", "erin", "frank"}

// Users are n players, challenger first; n is at most 6.
func Users(n int) []game.UserID { return slices.Clone(userNames[:n]) }

// ViewJSON is v as the core sends it.
func ViewJSON(t testing.TB, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode view %T: %v", v, err)
	}
	return string(raw)
}

// MustApply is s after seat applies action, failing t unless the adapter
// would take action, as info catalogues it, from seat, and the rules accept
// it with an S as the next state.
func MustApply[S game.State[Action, View], Action, View any](t testing.TB, info game.Info, s S, seat game.Seat, action Action, rng *rand.Rand) (S, []game.Event) {
	t.Helper()
	if err := game.CheckCatalogued(info, action); err != nil {
		t.Fatalf("%s %+v: %v", seat, action, err)
	}
	if err := game.CheckTurn(s, seat); err != nil {
		t.Fatalf("%s %+v: %v", seat, action, err)
	}
	next, events, err := s.Apply(seat, action, rng)
	if err != nil {
		t.Fatalf("%s %+v: %v", seat, action, err)
	}
	typed, ok := next.(S)
	if !ok {
		t.Fatalf("%s %+v: the next state is a %T, not a %T", seat, action, next, s)
	}
	return typed, events
}

// PlayToFinish plays a game of users from seed, the first seat to act each
// step making choose's action for its view, with the rng the core gives
// that step, and returns the steps to its finish, of which there are at
// most maxSteps.
func PlayToFinish[Opts, Action, View any](t testing.TB, rules game.Rules[Opts, Action, View], opts Opts, users []game.UserID, seed game.Seed, choose func(View, game.Seat) Action, maxSteps int) []Step[Action] {
	t.Helper()
	state, _, err := rules.New(opts, users[0], users[1:], game.StepRNG(seed, 0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var steps []Step[Action]
	for seq := game.Seq(1); len(state.ToAct()) > 0; seq++ {
		if len(steps) == maxSteps {
			t.Fatalf("no finish within %d steps", maxSteps)
		}
		seat := state.ToAct()[0]
		action := choose(state.View(seat), seat)
		state, _ = MustApply(t, rules.Info(), state, seat, action, game.StepRNG(seed, seq))
		steps = append(steps, Step[Action]{Seat: seat, Action: action})
	}
	return steps
}

// Replay is first and every state after it as steps, such as PlayToFinish's,
// are applied in turn through MustApply with info, each with the rng the core
// gives that step of a game from seed.
func Replay[S game.State[Action, View], Action, View any](t testing.TB, info game.Info, first S, steps []Step[Action], seed game.Seed) []S {
	t.Helper()
	states := []S{first}
	for i, step := range steps {
		next, _ := MustApply(t, info, states[i], step.Seat, step.Action, game.StepRNG(seed, game.Seq(i+1)))
		states = append(states, next)
	}
	return states
}

// SeatsFirstMover checks that rules, created with opts of each first mover,
// seat the player who moves first in first, which is to act: the challenger
// for FirstMoverMe, the opponent for FirstMoverOpponent, and for
// FirstMoverRandom the opponent from seed 1 and the challenger from seed 6.
func SeatsFirstMover[Opts, Action, View any](t *testing.T, rules game.Rules[Opts, Action, View], opts func(game.FirstMover) Opts, first game.Seat) {
	t.Helper()
	challenger, opponent := userNames[0], userNames[1]
	tests := []struct {
		first game.FirstMover
		seed  game.Seed
		want  game.UserID
	}{
		{game.FirstMoverMe, 1, challenger},
		{game.FirstMoverOpponent, 1, opponent},
		{game.FirstMoverRandom, 1, opponent},
		{game.FirstMoverRandom, 6, challenger},
	}
	for _, tc := range tests {
		state, seats, err := rules.New(opts(tc.first), challenger, []game.UserID{opponent}, game.StepRNG(tc.seed, 0))
		if err != nil {
			t.Fatalf("%s seed %d: %v", tc.first, tc.seed, err)
		}
		if seats[tc.want] != first || len(seats) != 2 || seats[challenger] == seats[opponent] {
			t.Errorf("%s seed %d: seats %v, want %s %s", tc.first, tc.seed, seats, tc.want, first)
		}
		if !slices.Contains(state.ToAct(), first) {
			t.Errorf("%s seed %d: to act %v, want %s among them", tc.first, tc.seed, state.ToAct(), first)
		}
	}
}
