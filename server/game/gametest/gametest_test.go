package gametest

import (
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

func TestRunCounterRules(t *testing.T) {
	Run(t, CounterRules{}, counterScripts)
	Run(t, ScoredCounterRules{}, counterScripts)
}

var counterScripts = []Script[CounterOpts, CounterAction]{
	{
		Name:       "accepted and rejected steps",
		Opts:       CounterOpts{Step: "2"},
		Challenger: "u",
		Seed:       1,
		Steps: []Step[CounterAction]{
			{Seat: "a", Action: CounterAction{Add: 3}},
			{Seat: "a", Action: CounterAction{Add: -1}, WantErr: true},
			{Seat: "a", Action: CounterAction{Add: 1}},
		},
	},
	{
		Name:       "two players alternate until one reaches the target",
		Challenger: "u",
		Invitees:   []game.UserID{"v"},
		Seed:       1,
		Steps: []Step[CounterAction]{
			{Seat: "b", Action: CounterAction{Add: 1}, WantErr: true},
			{Seat: "a", Action: CounterAction{Add: 5}},
			{Seat: "b", Action: CounterAction{Add: 5}},
			{Seat: "a", Action: CounterAction{Add: 1}, WantErr: true},
		},
		WantResult: &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{"b"}, Reason: ReasonTargetReached},
	},
}
