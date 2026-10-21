package game_test

import (
	"encoding/json"
	"errors"
	"maps"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const optStep game.OptionName = "step"

func resolve(t *testing.T, raw string) (game.Options, error) {
	t.Helper()
	return game.Adapt(gametest.CounterRules{}).ResolveOptions(json.RawMessage(raw))
}

func TestResolveOptionsRejectsUnknownValue(t *testing.T) {
	for _, raw := range []string{`{"step":"9"}`, `{"bogus":"1"}`} {
		_, err := resolve(t, raw)
		if !errors.As(err, &game.BadRequestError{}) {
			t.Errorf("%s: want game.BadRequestError, got %v", raw, err)
		}
	}
}

type notedCounterRules struct{ gametest.CounterRules }

const noteMaxLen = 8

func (notedCounterRules) Info() game.Info {
	info := gametest.CounterRules{}.Info()
	info.Options = append(slices.Clone(info.Options), game.TextOption("note", noteMaxLen))
	return info
}

func TestResolveOptionsChecksTextOptions(t *testing.T) {
	g := game.Adapt(notedCounterRules{})
	accepted := map[string]struct {
		raw  string
		want game.Options
	}{
		"defaults to empty":        {raw: `{}`, want: game.Options{optStep: "1", "note": ""}},
		"echoes text at the limit": {raw: `{"note":"ab cd 12"}`, want: game.Options{optStep: "1", "note": "ab cd 12"}},
	}
	for name, tc := range accepted {
		got, err := g.ResolveOptions(json.RawMessage(tc.raw))
		if err != nil || !maps.Equal(got, tc.want) {
			t.Errorf("%s: got %v, %v; want %v", name, got, err, tc.want)
		}
	}
	rejected := map[string]string{
		"text over the limit":    `{"note":"ab cd 123"}`,
		"newline":                `{"note":"a\nb"}`,
		"tab":                    `{"note":"a\tb"}`,
		"NUL":                    `{"note":"a\u0000b"}`,
		"DEL":                    `{"note":"a\u007fb"}`,
		"C1 next line":           `{"note":"a\u0085b"}`,
		"right-to-left override": `{"note":"a\u202eb"}`,
		"text for an enum":       `{"step":"ab"}`,
		"null":                   `{"note":null}`,
		"number":                 `{"note":1}`,
		"nested object":          `{"note":{"a":"b"}}`,
	}
	for name, raw := range rejected {
		got, err := g.ResolveOptions(json.RawMessage(raw))
		if !errors.As(err, &game.BadRequestError{}) {
			t.Errorf("%s: want game.BadRequestError, got %v, %v", name, got, err)
		}
	}
}

func TestCheckOptionsRejectsInvalidSpecs(t *testing.T) {
	enum := game.OptionSpec{Name: "e", Values: game.OptionValues("x", "y"), Default: "x"}
	if err := (game.Info{Options: []game.OptionSpec{enum, game.TextOption("t", 4)}}).CheckOptions(); err != nil {
		t.Fatalf("valid specs: %v", err)
	}
	longDefault := game.TextOption("t", 4)
	longDefault.Default = "12345"
	withValues := game.TextOption("t", 4)
	withValues.Values = game.OptionValues("x")
	for name, spec := range map[string]game.OptionSpec{
		"enum default not a value": {Name: "e", Values: game.OptionValues("x"), Default: "z"},
		"text default too long":    longDefault,
		"text with values":         withValues,
		"unknown kind":             {Name: "k", Kind: game.OptionKind(9)},
	} {
		if err := (game.Info{Options: []game.OptionSpec{spec}}).CheckOptions(); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestResolveOptionsFillsDefaults(t *testing.T) {
	g := game.Adapt(gametest.CounterRules{})
	opts, err := g.ResolveOptions(json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := (game.Options{optStep: "1"}); !maps.Equal(opts, want) {
		t.Fatalf("got %v want %v", opts, want)
	}
	st, _, err := g.New(opts, "u", nil, game.StepRNG(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	st, _, err = st.Apply("a", json.RawMessage(`{"Add":5}`), game.StepRNG(1, 1))
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
	got, err := game.Options{game.OptionTimeControl: "10+5"}.Clock()
	if err != nil || got != (game.ClockConfig{Initial: 10 * time.Minute, Increment: 5 * time.Second, Enabled: true}) {
		t.Errorf("10+5: got %+v, %v", got, err)
	}
	for _, o := range []game.Options{{game.OptionTimeControl: "none"}, {}} {
		got, err := o.Clock()
		if err != nil || got.Enabled {
			t.Errorf("%v: got %+v, %v", o, got, err)
		}
	}
	if _, err := (game.Options{game.OptionTimeControl: "x+y"}).Clock(); err == nil {
		t.Error("x+y: want error")
	}
}

func TestAdaptEncodesEventsWithKind(t *testing.T) {
	st, _, err := game.Adapt(gametest.CounterRules{}).New(game.Options{optStep: "1"}, "u", nil, game.StepRNG(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	_, events, err := st.Apply("a", json.RawMessage(`{"Add":5}`), game.StepRNG(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"by":"a","kind":"counted","total":5}`; len(events) != 1 || string(events[0]) != want {
		t.Fatalf("events %s, want [%s]", events, want)
	}
}

// unguardedRules play any action from any seat, leaving the turn to the adapter.
type unguardedRules struct{}

type unguardedState struct{ game.Ending }

func (unguardedRules) Info() game.Info {
	return game.Info{Type: "unguarded", Players: game.PlayerRange{Min: 1, Max: 2}}
}

func (unguardedRules) New(_ struct{}, challenger game.UserID, _ []game.UserID, _ *rand.Rand) (game.State[gametest.CounterAction, struct{}], map[game.UserID]game.Seat, error) {
	return unguardedState{}, map[game.UserID]game.Seat{challenger: "a"}, nil
}

func (s unguardedState) Apply(game.Seat, gametest.CounterAction, *rand.Rand) (game.State[gametest.CounterAction, struct{}], []game.Event, error) {
	return s, nil, nil
}

func (unguardedState) View(game.Seat) struct{} { return struct{}{} }
func (unguardedState) ToAct() []game.Seat      { return []game.Seat{"a"} }
func (s unguardedState) Eliminate(game.Seat, game.ResultReason) (game.State[gametest.CounterAction, struct{}], []game.Event) {
	return s, nil
}

func TestAdaptDecodesAnActionBeforeCheckingTheTurn(t *testing.T) {
	st, _, err := game.Adapt(unguardedRules{}).New(game.Options{}, "u", nil, game.StepRNG(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		seat    game.Seat
		action  string
		wantErr func(error) bool
	}{
		"the seat to act, malformed":    {"a", `"not json"`, isBadRequest},
		"another seat, malformed":       {"b", `"not json"`, isBadRequest},
		"another seat, well formed":     {"b", `{"Add":1}`, func(err error) bool { return errors.Is(err, game.ErrNotYourTurn) }},
		"a seat not in the game at all": {"z", `{"Add":1}`, func(err error) bool { return errors.Is(err, game.ErrNotYourTurn) }},
	} {
		if _, _, err := st.Apply(tc.seat, json.RawMessage(tc.action), game.StepRNG(1, 1)); !tc.wantErr(err) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestAdaptSeatsOnlyThePlayersTheGameTakes(t *testing.T) {
	for name, tc := range map[string]struct {
		rules    game.Game
		invitees []game.UserID
		wantErr  bool
	}{
		"counter alone":        {game.Adapt(gametest.CounterRules{}), nil, false},
		"counter for two":      {game.Adapt(gametest.CounterRules{}), []game.UserID{"v"}, false},
		"counter for three":    {game.Adapt(gametest.CounterRules{}), []game.UserID{"v", "w"}, true},
		"solo counter for two": {game.Adapt(gametest.SoloCounterRules{}), []game.UserID{"v"}, true},
	} {
		_, _, err := tc.rules.New(game.Options{optStep: "1", "note": ""}, "u", tc.invitees, game.StepRNG(1, 1))
		if tc.wantErr != isBadRequest(err) || !tc.wantErr && err != nil {
			t.Errorf("%s: got %v, want a bad request %t", name, err, tc.wantErr)
		}
	}
}

func isBadRequest(err error) bool { return errors.As(err, &game.BadRequestError{}) }

func TestStepRNGDeterministic(t *testing.T) {
	if game.StepRNG(7, 3).Uint64() != game.StepRNG(7, 3).Uint64() {
		t.Error("same (seed, seq) must repeat")
	}
	if game.StepRNG(7, 3).Uint64() == game.StepRNG(7, 4).Uint64() {
		t.Error("different seq must differ")
	}
}

func TestForfeitToRemaining(t *testing.T) {
	for name, tc := range map[string]struct{ all, want []game.Seat }{
		"two players: the other wins": {all: []game.Seat{"w", "b"}, want: []game.Seat{"b"}},
		"solo: lost with no winner":   {all: []game.Seat{"w"}, want: []game.Seat{}},
	} {
		got := game.ForfeitToRemaining(tc.all, "w", game.ReasonResign)
		if got.Kind != game.ResultDecisive || got.Winners == nil || !slices.Equal(got.Winners, tc.want) || got.Reason != game.ReasonResign {
			t.Errorf("%s: got %+v", name, got)
		}
	}
}

type cataloguedCounterRules struct{ gametest.CounterRules }

func (cataloguedCounterRules) EventTypes() []game.Event { return []game.Event{gametest.CountedEvent{}} }
func (cataloguedCounterRules) Reasons() []game.ResultReason {
	return []game.ResultReason{gametest.ReasonTargetReached}
}

type declaredCounterRules struct{ cataloguedCounterRules }

func (declaredCounterRules) Info() game.Info {
	info := gametest.CounterRules{}.Info()
	info.Events = []game.Event{gametest.CountedEvent{}, gametest.CountedEvent{}}
	info.Reasons = []game.ResultReason{"declared"}
	return info
}

func TestAdaptFillsTheInfoFromTheRulesCatalogs(t *testing.T) {
	action, view := game.Adapt(gametest.CounterRules{}).WireTypes()
	if action != reflect.TypeFor[gametest.CounterAction]() || view != reflect.TypeFor[gametest.CounterView]() {
		t.Errorf("wire types %v, %v", action, view)
	}
	for name, tc := range map[string]struct {
		rules       game.Game
		wantEvents  int
		wantReasons []game.ResultReason
	}{
		"no catalogs":                  {game.Adapt(gametest.CounterRules{}), 0, nil},
		"catalogs":                     {game.Adapt(cataloguedCounterRules{}), 1, []game.ResultReason{gametest.ReasonTargetReached}},
		"the Info's own over catalogs": {game.Adapt(declaredCounterRules{}), 2, []game.ResultReason{"declared"}},
	} {
		info := tc.rules.Info()
		if len(info.Events) != tc.wantEvents || !slices.Equal(info.Reasons, tc.wantReasons) {
			t.Errorf("%s: events %v, reasons %v", name, info.Events, info.Reasons)
		}
	}
}

func TestAdaptTicksOnlyATickingState(t *testing.T) {
	counter, _, err := game.Adapt(gametest.CounterRules{}).New(game.Options{optStep: "1"}, "u", nil, game.StepRNG(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, due := counter.NextTick(); due || counter.RealTime() {
		t.Errorf("a counter, which does not tick, wants a tick (%t) or is real-time (%t)", due, counter.RealTime())
	}
	if _, _, err := counter.Tick(game.StepRNG(1, 1)); err == nil {
		t.Error("ticking a counter succeeded")
	}

	metronome, _, err := game.Adapt(gametest.MetronomeRules{}).New(game.Options{}, "u", nil, game.StepRNG(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if d, due := metronome.NextTick(); d != gametest.MetronomeInterval || !due {
		t.Errorf("metronome NextTick = %v, %t; want %v, true", d, due, gametest.MetronomeInterval)
	}
	ticked, events, err := metronome.Tick(game.StepRNG(1, 1))
	if err != nil || len(events) != 1 || !strings.Contains(string(events[0]), `"kind":"beat"`) {
		t.Fatalf("metronome tick events %s, %v; want one beat", events, err)
	}
	if _, due := ticked.Pause().NextTick(); due || !ticked.Pause().RealTime() {
		t.Errorf("a paused metronome wants a tick (%t) or is not real-time (%t)", due, ticked.Pause().RealTime())
	}
	if _, due := ticked.NextTick(); !due {
		t.Error("pausing changed the state it paused")
	}
}
