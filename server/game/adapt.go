package game

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
)

type adapted[Opts, Action, View any] struct {
	rules Rules[Opts, Action, View]
}

// Adapt wraps typed rules so JSON encoding and decoding happen once, here.
func Adapt[Opts, Action, View any](rules Rules[Opts, Action, View]) Game {
	return adapted[Opts, Action, View]{rules: rules}
}

func (g adapted[Opts, Action, View]) Info() Info { return g.rules.Info() }

func (g adapted[Opts, Action, View]) ResolveOptions(raw json.RawMessage) (Options, error) {
	given := Options{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &given); err != nil {
			return nil, BadRequestError{Msg: fmt.Sprintf("options must be an object of strings: %v", err)}
		}
	}
	specs := g.rules.Info().Options
	for name, value := range given {
		i := slices.IndexFunc(specs, func(s OptionSpec) bool { return s.Name == name })
		if i < 0 {
			return nil, BadRequestError{Msg: fmt.Sprintf("unknown option %q", name)}
		}
		if !slices.Contains(specs[i].Values, value) {
			return nil, BadRequestError{Msg: fmt.Sprintf("option %q: invalid value %q, want one of %v", name, value, specs[i].Values)}
		}
	}
	resolved := make(Options, len(specs))
	for _, spec := range specs {
		value, ok := given[spec.Name]
		if !ok {
			value = spec.Default
		}
		resolved[spec.Name] = value
	}
	return resolved, nil
}

func (g adapted[Opts, Action, View]) New(opts Options, challenger UserID, invitees []UserID, rng *rand.Rand) (ErasedState, map[UserID]Seat, error) {
	encoded, err := json.Marshal(opts)
	if err != nil {
		return nil, nil, err
	}
	var typed Opts
	if err := json.Unmarshal(encoded, &typed); err != nil {
		return nil, nil, fmt.Errorf("decode options: %w", err)
	}
	state, seats, err := g.rules.New(typed, challenger, invitees, rng)
	if err != nil {
		return nil, nil, err
	}
	return erased[Action, View]{state: state}, seats, nil
}

type erased[Action, View any] struct {
	state State[Action, View]
}

func (e erased[Action, View]) Apply(seat Seat, raw json.RawMessage, rng *rand.Rand) (ErasedState, []Event, error) {
	var action Action
	if err := json.Unmarshal(raw, &action); err != nil {
		return nil, nil, BadRequestError{Msg: fmt.Sprintf("malformed action: %v", err)}
	}
	next, events, err := e.state.Apply(seat, action, rng)
	if err != nil {
		return nil, nil, err
	}
	return erased[Action, View]{state: next}, events, nil
}

func (e erased[Action, View]) View(seat Seat) (json.RawMessage, error) {
	return json.Marshal(e.state.View(seat))
}

func (e erased[Action, View]) ToAct() []Seat           { return e.state.ToAct() }
func (e erased[Action, View]) Outcome() (Result, bool) { return e.state.Outcome() }

func (e erased[Action, View]) Eliminate(seat Seat, reason ResultReason) (ErasedState, []Event) {
	next, events := e.state.Eliminate(seat, reason)
	return erased[Action, View]{state: next}, events
}
