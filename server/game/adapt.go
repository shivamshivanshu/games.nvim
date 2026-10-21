package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"time"
)

type adapted[Opts, Action, View any] struct {
	rules Rules[Opts, Action, View]
}

func Adapt[Opts, Action, View any](rules Rules[Opts, Action, View]) Game {
	return adapted[Opts, Action, View]{rules: rules}
}

func (g adapted[Opts, Action, View]) Info() Info { return g.rules.Info() }

func (g adapted[Opts, Action, View]) WireTypes() (action, view reflect.Type) {
	return reflect.TypeFor[Action](), reflect.TypeFor[View]()
}

func (g adapted[Opts, Action, View]) EventCatalog() (EventCatalog, bool) {
	catalog, ok := any(g.rules).(EventCatalog)
	return catalog, ok
}

func (g adapted[Opts, Action, View]) ReasonCatalog() (ReasonCatalog, bool) {
	catalog, ok := any(g.rules).(ReasonCatalog)
	return catalog, ok
}

func (g adapted[Opts, Action, View]) ResolveOptions(raw json.RawMessage) (Options, error) {
	given, err := decodeGivenOptions(raw)
	if err != nil {
		return nil, err
	}
	specs := g.rules.Info().Options
	for name, value := range given {
		i := slices.IndexFunc(specs, func(s OptionSpec) bool { return s.Name == name })
		if i < 0 {
			return nil, BadRequestError{Msg: fmt.Sprintf("unknown option %q", name)}
		}
		if err := specs[i].check(value); err != nil {
			return nil, err
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
	if _, err := resolved.Clock(); err != nil {
		return nil, BadRequestError{Msg: err.Error()}
	}
	return resolved, nil
}

// decodeGivenOptions decodes into pointers so that a JSON null is told apart
// from "" and rejected like any other non-string.
func decodeGivenOptions(raw json.RawMessage) (Options, error) {
	var given map[OptionName]*OptionValue
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &given); err != nil {
			return nil, BadRequestError{Msg: fmt.Sprintf("options must be an object of strings: %v", err)}
		}
	}
	out := make(Options, len(given))
	for name, value := range given {
		if value == nil {
			return nil, BadRequestError{Msg: fmt.Sprintf("option %q must be a string, got null", name)}
		}
		out[name] = *value
	}
	return out, nil
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
	opening, err := encodeEvents(openingEvents(state))
	if err != nil {
		return nil, nil, err
	}
	return erased[Action, View]{state: state, opening: opening}, seats, nil
}

func openingEvents(state any) []Event {
	opener, ok := state.(Opener)
	if !ok {
		return nil
	}
	return opener.OpeningEvents()
}

type erased[Action, View any] struct {
	state   State[Action, View]
	opening []json.RawMessage
}

func (e erased[Action, View]) OpeningEvents() []json.RawMessage {
	return append([]json.RawMessage{}, e.opening...)
}

func (e erased[Action, View]) Apply(seat Seat, raw json.RawMessage, rng *rand.Rand) (ErasedState, []json.RawMessage, error) {
	var action Action
	if err := json.Unmarshal(raw, &action); err != nil {
		return nil, nil, BadRequestError{Msg: fmt.Sprintf("malformed action: %v", err)}
	}
	next, events, err := e.state.Apply(seat, action, rng)
	if err != nil {
		return nil, nil, err
	}
	return erasedWithEvents(next, events)
}

func (e erased[Action, View]) View(seat Seat) (json.RawMessage, error) {
	return json.Marshal(e.state.View(seat))
}

func (e erased[Action, View]) ToAct() []Seat           { return e.state.ToAct() }
func (e erased[Action, View]) Outcome() (Result, bool) { return e.state.Outcome() }

func (e erased[Action, View]) KnockedOut() []Seat {
	knockout, ok := e.state.(Knockout)
	if !ok {
		return []Seat{}
	}
	return knockout.KnockedOut()
}

func (e erased[Action, View]) RealTime() bool {
	_, ok := e.state.(Ticking[Action, View])
	return ok
}

func (e erased[Action, View]) NextTick() (time.Duration, bool) {
	ticking, ok := e.state.(Ticking[Action, View])
	if !ok {
		return 0, false
	}
	return ticking.NextTick()
}

func (e erased[Action, View]) Tick(rng *rand.Rand) (ErasedState, []json.RawMessage, error) {
	ticking, ok := e.state.(Ticking[Action, View])
	if !ok {
		return nil, nil, errors.New("game: tick of a state that does not tick")
	}
	next, events := ticking.Tick(rng)
	return erasedWithEvents(next, events)
}

func (e erased[Action, View]) Pause() ErasedState {
	ticking, ok := e.state.(Ticking[Action, View])
	if !ok {
		return erased[Action, View]{state: e.state}
	}
	return erased[Action, View]{state: ticking.Pause()}
}

func (e erased[Action, View]) Scores() (map[Seat]int64, bool) {
	scorer, ok := e.state.(Scorer)
	if !ok {
		return nil, false
	}
	return scorer.Scores(), true
}

func (e erased[Action, View]) Exports(ctx ExportContext) map[string][]byte {
	exporter, ok := e.state.(Exporter)
	if !ok {
		return map[string][]byte{}
	}
	return exporter.Exports(ctx)
}

func (e erased[Action, View]) Conclude(result Result) ErasedState {
	concluder, ok := e.state.(Concluder[Action, View])
	if !ok {
		return erased[Action, View]{state: e.state}
	}
	return erased[Action, View]{state: concluder.Conclude(result)}
}

func (e erased[Action, View]) Eliminate(seat Seat, reason ResultReason) (ErasedState, []json.RawMessage, error) {
	next, events := e.state.Eliminate(seat, reason)
	return erasedWithEvents(next, events)
}

func erasedWithEvents[Action, View any](next State[Action, View], events []Event) (ErasedState, []json.RawMessage, error) {
	encoded, err := encodeEvents(events)
	if err != nil {
		return nil, nil, err
	}
	return erased[Action, View]{state: next}, encoded, nil
}

func encodeEvents(events []Event) ([]json.RawMessage, error) {
	encoded := make([]json.RawMessage, 0, len(events))
	for _, e := range events {
		raw, err := EncodeEvent(e)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, raw)
	}
	return encoded, nil
}

// EventKindField is the field of an encoded event that holds its kind.
const EventKindField = "kind"

// EncodeEvent is the wire form of e: its JSON object with the event kind added.
func EncodeEvent(e Event) (json.RawMessage, error) {
	raw, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("encode %s event: %w", e.EventKind(), err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("event %s is not a JSON object: %w", e.EventKind(), err)
	}
	kind, err := json.Marshal(e.EventKind())
	if err != nil {
		return nil, err
	}
	if fields == nil {
		fields = map[string]json.RawMessage{}
	}
	fields[EventKindField] = kind
	return json.Marshal(fields)
}
