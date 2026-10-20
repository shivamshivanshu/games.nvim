package gametest

import "testing"

func TestRunCounterRules(t *testing.T) {
	Run(t, CounterRules{}, []Script[CounterOpts, CounterAction]{
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
	})
}
