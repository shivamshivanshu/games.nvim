package gametest

import (
	"math/rand/v2"
	"testing"

	"games.nvim/server/game"
)

type counterOpts struct {
	Step string `json:"step"`
}

type counterAction struct{ Add int }

type counterView struct{ Total int }

type counterState struct{ total, step int }

func (s counterState) Apply(seat game.Seat, a counterAction, _ *rand.Rand) (game.State[counterAction, counterView], []game.Event, error) {
	if a.Add < 0 {
		return nil, nil, game.IllegalActionError{Msg: "negative add"}
	}
	s.total += a.Add * s.step
	return s, nil, nil
}
func (s counterState) View(game.Seat) counterView   { return counterView{Total: s.total} }
func (s counterState) ToAct() []game.Seat           { return []game.Seat{"a"} }
func (s counterState) Outcome() (game.Result, bool) { return game.Result{}, false }
func (s counterState) Eliminate(game.Seat, game.ResultReason) (game.State[counterAction, counterView], []game.Event) {
	return s, nil
}

type counterRules struct{}

func (counterRules) Info() game.Info {
	return game.Info{
		Type:    "counter",
		Options: []game.OptionSpec{{Name: "step", Values: []string{"1", "2"}, Default: "1"}},
	}
}

func (counterRules) New(o counterOpts, challenger game.UserID, _ []game.UserID, _ *rand.Rand) (game.State[counterAction, counterView], map[game.UserID]game.Seat, error) {
	step := 1
	if o.Step == "2" {
		step = 2
	}
	return counterState{step: step}, map[game.UserID]game.Seat{challenger: "a"}, nil
}

func TestRunCounterRules(t *testing.T) {
	Run(t, counterRules{}, []Script[counterOpts, counterAction]{
		{
			Name:       "accepted and rejected steps",
			Opts:       counterOpts{Step: "2"},
			Challenger: "u",
			Seed:       1,
			Steps: []Step[counterAction]{
				{Seat: "a", Action: counterAction{Add: 3}},
				{Seat: "a", Action: counterAction{Add: -1}, WantErr: true},
				{Seat: "a", Action: counterAction{Add: 1}},
			},
		},
	})
}
