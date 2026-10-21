package game_test

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type cataloguedAction struct {
	Kind game.ActionKind `json:"kind"`
	Cell string          `json:"cell,omitempty"`
	Way  string          `json:"way,omitempty"`
}

// cataloguedRules take any action their catalogue lets through.
type cataloguedRules struct{}

type cataloguedState struct{ game.Ending }

func (cataloguedRules) Info() game.Info {
	return game.Info{Type: "catalogued", Players: game.PlayerRange{Min: 1, Max: 1}, Actions: []game.ActionSpec{
		{Kind: "place", Input: &game.ActionInput{Field: "cell", Description: "a1 to c3.", Pattern: `^\s*[a-c][1-3]\s*$`}},
		{Kind: "turn", Input: &game.ActionInput{Field: "way", Values: []string{"up", "down"}}},
		{Kind: "stop"},
	}}
}

func (cataloguedRules) New(_ struct{}, challenger game.UserID, _ []game.UserID, _ *rand.Rand) (game.State[cataloguedAction, struct{}], map[game.UserID]game.Seat, error) {
	return cataloguedState{}, map[game.UserID]game.Seat{challenger: "a"}, nil
}

func (s cataloguedState) Apply(game.Seat, cataloguedAction, *rand.Rand) (game.State[cataloguedAction, struct{}], []game.Event, error) {
	return s, nil, nil
}

func (cataloguedState) View(game.Seat) struct{} { return struct{}{} }
func (cataloguedState) ToAct() []game.Seat      { return []game.Seat{"a"} }
func (s cataloguedState) Eliminate(game.Seat, game.ResultReason) (game.State[cataloguedAction, struct{}], []game.Event) {
	return s, nil
}
func (s cataloguedState) Conclude(r game.Result) game.State[cataloguedAction, struct{}] {
	s.Ending = s.Concluded(r)
	return s
}

func isIllegal(err error) bool { return errors.As(err, &game.IllegalActionError{}) }

func TestAdaptTakesOnlyActionsTheCatalogueAllows(t *testing.T) {
	st, _, err := game.Adapt(cataloguedRules{}).New(game.Options{}, "u", nil, game.StepRNG(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	accepted := func(err error) bool { return err == nil }
	for name, tc := range map[string]struct {
		action  string
		wantErr func(error) bool
	}{
		"a matching input":           {`{"kind":"place","cell":"b2"}`, accepted},
		"a padded input":             {`{"kind":"place","cell":" b2 "}`, accepted},
		"one of the values":          {`{"kind":"turn","way":"up"}`, accepted},
		"a kind alone":               {`{"kind":"stop"}`, accepted},
		"a kind the game refuses":    {`{"kind":"jump"}`, accepted},
		"an input off its pattern":   {`{"kind":"place","cell":"d9"}`, isIllegal},
		"an input not of its values": {`{"kind":"turn","way":"left"}`, isIllegal},
		"a padded value":             {`{"kind":"turn","way":"up "}`, isIllegal},
		"a missing input":            {`{"kind":"place"}`, isIllegal},
		"a field the action lacks":   {`{"kind":"place","cell":"b2","extra":"x"}`, isBadRequest},
		"another kind's input":       {`{"kind":"stop","cell":"b2"}`, isBadRequest},
		"an input that is no text":   {`{"kind":"place","cell":2}`, isBadRequest},
		"a kind in another case":     {`{"KIND":"place","cell":"b2"}`, isBadRequest},
		"an input in another case":   {`{"kind":"place","Cell":"b2"}`, isBadRequest},
	} {
		if _, _, err := st.Apply("a", json.RawMessage(tc.action), game.StepRNG(1, 1)); !tc.wantErr(err) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestCheckCataloguedChecksATypedActionAsTheAdapterDoes(t *testing.T) {
	info := cataloguedRules{}.Info()
	accepted := func(err error) bool { return err == nil }
	for name, tc := range map[string]struct {
		action  cataloguedAction
		wantErr func(error) bool
	}{
		"a matching input":         {cataloguedAction{Kind: "place", Cell: "b2"}, accepted},
		"a kind alone":             {cataloguedAction{Kind: "stop"}, accepted},
		"an input off its pattern": {cataloguedAction{Kind: "place", Cell: "d9"}, isIllegal},
		"a padded value":           {cataloguedAction{Kind: "turn", Way: "up "}, isIllegal},
		"another kind's input":     {cataloguedAction{Kind: "stop", Cell: "b2"}, isBadRequest},
	} {
		if err := game.CheckCatalogued(info, tc.action); !tc.wantErr(err) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}
