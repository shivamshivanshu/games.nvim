package game

import (
	"encoding/json"
	"errors"
	"maps"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

type counterOpts struct {
	Step string `json:"step"`
}

type counterAction struct{ Add int }

type counterView struct{ Total int }

type counterState struct{ total, step int }

func (s counterState) Apply(seat Seat, a counterAction, _ *rand.Rand) (State[counterAction, counterView], []Event, error) {
	s.total += a.Add * s.step
	return s, nil, nil
}
func (s counterState) View(Seat) counterView   { return counterView{Total: s.total} }
func (s counterState) ToAct() []Seat           { return []Seat{"a"} }
func (s counterState) Outcome() (Result, bool) { return Result{}, false }
func (s counterState) Eliminate(Seat, ResultReason) (State[counterAction, counterView], []Event) {
	return s, nil
}

type counterRules struct{}

func (counterRules) Info() Info {
	return Info{
		Type:    "counter",
		Options: []OptionSpec{{Name: "step", Values: []string{"1", "2"}, Default: "1"}},
	}
}

func (counterRules) New(o counterOpts, challenger UserID, _ []UserID, _ *rand.Rand) (State[counterAction, counterView], map[UserID]Seat, error) {
	step := 1
	if o.Step == "2" {
		step = 2
	}
	return counterState{step: step}, map[UserID]Seat{challenger: "a"}, nil
}

func resolve(t *testing.T, raw string) (Options, error) {
	t.Helper()
	return Adapt(counterRules{}).ResolveOptions(json.RawMessage(raw))
}

func TestResolveOptionsRejectsUnknownValue(t *testing.T) {
	for _, raw := range []string{`{"step":"9"}`, `{"bogus":"1"}`} {
		_, err := resolve(t, raw)
		if !errors.As(err, &BadRequestError{}) {
			t.Errorf("%s: want BadRequestError, got %v", raw, err)
		}
	}
}

func TestResolveOptionsFillsDefaults(t *testing.T) {
	g := Adapt(counterRules{})
	opts, err := g.ResolveOptions(json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := (Options{"step": "1"}); !maps.Equal(opts, want) {
		t.Fatalf("got %v want %v", opts, want)
	}
	st, _, err := g.New(opts, "u", nil, StepRNG(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	st, _, err = st.Apply("a", json.RawMessage(`{"Add":5}`), StepRNG(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	view, err := st.View("a")
	if err != nil {
		t.Fatal(err)
	}
	if string(view) != `{"Total":5}` {
		t.Fatalf("view %s reflects wrong step", view)
	}
}

func TestOptionsClock(t *testing.T) {
	got, err := Options{OptionTimeControl: "10+5"}.Clock()
	if err != nil || got != (ClockConfig{Initial: 10 * time.Minute, Increment: 5 * time.Second, Enabled: true}) {
		t.Errorf("10+5: got %+v, %v", got, err)
	}
	for _, o := range []Options{{OptionTimeControl: "none"}, {}} {
		got, err := o.Clock()
		if err != nil || got.Enabled {
			t.Errorf("%v: got %+v, %v", o, got, err)
		}
	}
	if _, err := (Options{OptionTimeControl: "x+y"}).Clock(); err == nil {
		t.Error("x+y: want error")
	}
}

func TestAdaptRejectsMalformedAction(t *testing.T) {
	g := Adapt(counterRules{})
	st, _, err := g.New(Options{"step": "1"}, "u", nil, StepRNG(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = st.Apply("a", json.RawMessage(`"not json"`), StepRNG(1, 1))
	if !errors.As(err, &BadRequestError{}) {
		t.Fatalf("want BadRequestError, got %v", err)
	}
}

func TestStepRNGDeterministic(t *testing.T) {
	if StepRNG(7, 3).Uint64() != StepRNG(7, 3).Uint64() {
		t.Error("same (seed, seq) must repeat")
	}
	if StepRNG(7, 3).Uint64() == StepRNG(7, 4).Uint64() {
		t.Error("different seq must differ")
	}
}

func TestForfeitToRemaining(t *testing.T) {
	got := ForfeitToRemaining([]Seat{"w", "b"}, "w", ReasonResign)
	if got.Kind != ResultDecisive || !slices.Equal(got.Winners, []Seat{"b"}) || got.Reason != ReasonResign {
		t.Fatalf("got %+v", got)
	}
}
