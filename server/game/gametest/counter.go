package gametest

import (
	"math/rand/v2"

	"games.nvim/server/game"
)

// CounterRules is a minimal game for testing the harness and the adapter:
// one seat "a" adds to a total, scaled by the "step" option.
type CounterRules struct{}

type CounterOpts struct {
	Step string `json:"step"`
}

type CounterAction struct{ Add int }

type CounterView struct{ Total int }

type CountedEvent struct {
	By    game.Seat `json:"by"`
	Total int       `json:"total"`
}

func (CountedEvent) EventKind() game.EventKind { return "counted" }

func (CounterRules) Info() game.Info {
	return game.Info{
		Type:    "counter",
		Options: []game.OptionSpec{{Name: "step", Values: []game.OptionValue{"1", "2"}, Default: "1"}},
	}
}

func (CounterRules) New(o CounterOpts, challenger game.UserID, _ []game.UserID, _ *rand.Rand) (game.State[CounterAction, CounterView], map[game.UserID]game.Seat, error) {
	step := 1
	if o.Step == "2" {
		step = 2
	}
	return counterState{step: step}, map[game.UserID]game.Seat{challenger: "a"}, nil
}

type counterState struct{ total, step int }

func (s counterState) Apply(seat game.Seat, a CounterAction, _ *rand.Rand) (game.State[CounterAction, CounterView], []game.Event, error) {
	if a.Add < 0 {
		return nil, nil, game.IllegalActionError{Msg: "negative add"}
	}
	s.total += a.Add * s.step
	return s, []game.Event{CountedEvent{By: seat, Total: s.total}}, nil
}

func (s counterState) View(game.Seat) CounterView   { return CounterView{Total: s.total} }
func (s counterState) ToAct() []game.Seat           { return []game.Seat{"a"} }
func (s counterState) Outcome() (game.Result, bool) { return game.Result{}, false }
func (s counterState) Eliminate(game.Seat, game.ResultReason) (game.State[CounterAction, CounterView], []game.Event) {
	return s, nil
}
