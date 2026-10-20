package gametest

import (
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// CounterRules is a minimal game for testing the harness, the adapter and the
// server core: the challenger (seat "a") and an optional invitee (seat "b")
// take turns adding to a total, scaled by the "step" option; whoever brings
// it to CounterTarget wins.
type CounterRules struct{}

// ScoredCounterRules is CounterRules whose final state is a game.Scorer: each
// seat scores what it added beyond its share, so the scores sum to zero.
type ScoredCounterRules struct{}

// SoloCounterRules is CounterRules for the challenger alone, a solo game, with
// a free-text "note" option of at most SoloNoteMaxLen bytes that play ignores.
type SoloCounterRules struct{ CounterRules }

const (
	CounterTarget                         = 10
	SoloNoteMaxLen                        = 8
	ReasonTargetReached game.ResultReason = "target_reached"
)

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
		Type:         "counter",
		Players:      game.PlayerRange{Min: 1, Max: 2},
		Capabilities: []game.Capability{game.CapResign},
		Options:      []game.OptionSpec{{Name: "step", Values: []game.OptionValue{"1", "2"}, Default: "1"}},
	}
}

func (CounterRules) New(o CounterOpts, challenger game.UserID, invitees []game.UserID, _ *rand.Rand) (game.State[CounterAction, CounterView], map[game.UserID]game.Seat, error) {
	state, seats := newCounter(o, challenger, invitees)
	return state, seats, nil
}

func (ScoredCounterRules) Info() game.Info {
	info := CounterRules{}.Info()
	info.Type = "scored_counter"
	return info
}

func (ScoredCounterRules) New(o CounterOpts, challenger game.UserID, invitees []game.UserID, _ *rand.Rand) (game.State[CounterAction, CounterView], map[game.UserID]game.Seat, error) {
	state, seats := newCounter(o, challenger, invitees)
	return scoredCounterState{state}, seats, nil
}

func (SoloCounterRules) Info() game.Info {
	info := CounterRules{}.Info()
	info.Type = "solo_counter"
	info.Players = game.PlayerRange{Min: 1, Max: 1}
	info.Options = append(slices.Clone(info.Options), game.TextOption("note", SoloNoteMaxLen))
	return info
}

func newCounter(o CounterOpts, challenger game.UserID, invitees []game.UserID) (counterState, map[game.UserID]game.Seat) {
	step := 1
	if o.Step == "2" {
		step = 2
	}
	seats := map[game.UserID]game.Seat{challenger: "a"}
	order := []game.Seat{"a"}
	if len(invitees) > 0 {
		seats[invitees[0]] = "b"
		order = append(order, "b")
	}
	return counterState{step: step, order: order, added: make([]int, len(order))}, seats
}

type counterState struct {
	total, step, turn int
	order             []game.Seat
	added             []int
	result            *game.Result
}

func (s counterState) Apply(seat game.Seat, a CounterAction, _ *rand.Rand) (game.State[CounterAction, CounterView], []game.Event, error) {
	next, events, err := s.apply(seat, a)
	if err != nil {
		return nil, nil, err
	}
	return next, events, nil
}

func (s counterState) apply(seat game.Seat, a CounterAction) (counterState, []game.Event, error) {
	switch {
	case s.result != nil:
		return s, nil, game.IllegalActionError{Msg: "game is over"}
	case seat != s.order[s.turn]:
		return s, nil, game.ErrNotYourTurn
	case a.Add < 0:
		return s, nil, game.IllegalActionError{Msg: "negative add"}
	}
	s.total += a.Add * s.step
	s.added = slices.Clone(s.added)
	s.added[s.turn] += a.Add * s.step
	s.turn = (s.turn + 1) % len(s.order)
	if s.total >= CounterTarget {
		s.result = &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat}, Reason: ReasonTargetReached}
	}
	return s, []game.Event{CountedEvent{By: seat, Total: s.total}}, nil
}

func (s counterState) View(game.Seat) CounterView { return CounterView{Total: s.total} }

func (s counterState) ToAct() []game.Seat {
	if s.result != nil {
		return []game.Seat{}
	}
	return []game.Seat{s.order[s.turn]}
}

func (s counterState) Outcome() (game.Result, bool) {
	if s.result == nil {
		return game.Result{}, false
	}
	return *s.result, true
}

func (s counterState) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[CounterAction, CounterView], []game.Event) {
	return s.eliminate(seat, reason), nil
}

func (s counterState) eliminate(seat game.Seat, reason game.ResultReason) counterState {
	if s.result == nil {
		s.result = new(game.ForfeitToRemaining(s.order, seat, reason))
	}
	return s
}

type scoredCounterState struct{ counterState }

func (s scoredCounterState) Apply(seat game.Seat, a CounterAction, _ *rand.Rand) (game.State[CounterAction, CounterView], []game.Event, error) {
	next, events, err := s.apply(seat, a)
	if err != nil {
		return nil, nil, err
	}
	return scoredCounterState{next}, events, nil
}

func (s scoredCounterState) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[CounterAction, CounterView], []game.Event) {
	return scoredCounterState{s.eliminate(seat, reason)}, nil
}

func (s scoredCounterState) Scores() map[game.Seat]int64 {
	scores := make(map[game.Seat]int64, len(s.order))
	for i, seat := range s.order {
		scores[seat] = int64(len(s.order)*s.added[i] - s.total)
	}
	return scores
}
