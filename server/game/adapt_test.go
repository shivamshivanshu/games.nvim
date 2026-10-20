package game_test

import (
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

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

func TestResolveOptionsFillsDefaults(t *testing.T) {
	g := game.Adapt(gametest.CounterRules{})
	opts, err := g.ResolveOptions(json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := (game.Options{"step": "1"}); !maps.Equal(opts, want) {
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
	st, _, err := game.Adapt(gametest.CounterRules{}).New(game.Options{"step": "1"}, "u", nil, game.StepRNG(1, 1))
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

func TestAdaptRejectsMalformedAction(t *testing.T) {
	g := game.Adapt(gametest.CounterRules{})
	st, _, err := g.New(game.Options{"step": "1"}, "u", nil, game.StepRNG(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = st.Apply("a", json.RawMessage(`"not json"`), game.StepRNG(1, 1))
	if !errors.As(err, &game.BadRequestError{}) {
		t.Fatalf("want game.BadRequestError, got %v", err)
	}
}

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
