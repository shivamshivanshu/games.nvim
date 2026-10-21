package gametest

import (
	"math/rand/v2"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// MetronomeRules is a minimal game.Ticking game for testing the harness, the
// adapter and the server core: the challenger plays alone while every tick,
// MetronomeInterval apart, adds 1 to 3 from the tick's rng to a total, until
// it reaches MetronomeTarget. The player may pause it, resume it, and tap,
// which only counts the taps.
type MetronomeRules struct{}

const (
	// MetronomeInterval is the time between ticks.
	MetronomeInterval = 100 * time.Millisecond
	// MetronomeTarget is the total that ends the game, won.
	MetronomeTarget = 12
	// MetronomeSeat is the only seat.
	MetronomeSeat game.Seat = "solo"
)

// The metronome's actions.
const (
	MetronomePause  game.ActionKind = "pause"
	MetronomeResume game.ActionKind = "resume"
	MetronomeTap    game.ActionKind = "tap"
)

// MetronomeAction is a pause, a resume or a tap.
type MetronomeAction struct {
	Kind game.ActionKind `json:"kind"`
}

// MetronomeView is the total so far, the ticks that made it, the taps, and whether it is paused.
type MetronomeView struct {
	Total  int  `json:"total"`
	Ticks  int  `json:"ticks"`
	Taps   int  `json:"taps"`
	Paused bool `json:"paused"`
}

// BeatEvent reports a tick and the total it reached.
type BeatEvent struct {
	Total int `json:"total"`
}

// EventKind implements game.Event.
func (BeatEvent) EventKind() game.EventKind { return "beat" }

func (MetronomeRules) Info() game.Info {
	return game.Info{
		Type:         "metronome",
		Players:      game.PlayerRange{Min: 1, Max: 1},
		Capabilities: []game.Capability{game.CapResign},
		Actions: []game.ActionSpec{
			{Kind: MetronomePause, Description: "Stop the ticks."},
			{Kind: MetronomeResume, Description: "Start the ticks again."},
			{Kind: MetronomeTap, Description: "Count a tap."},
		},
	}
}

func (MetronomeRules) New(_ struct{}, challenger game.UserID, invitees []game.UserID, _ *rand.Rand) (game.State[MetronomeAction, MetronomeView], map[game.UserID]game.Seat, error) {
	if len(invitees) > 0 {
		return nil, nil, game.BadRequestError{Msg: "metronome is solo"}
	}
	return metronomeState{}, map[game.UserID]game.Seat{challenger: MetronomeSeat}, nil
}

type metronomeState struct {
	total, ticks, taps int
	paused             bool
	result             *game.Result
}

func (s metronomeState) Apply(seat game.Seat, a MetronomeAction, _ *rand.Rand) (game.State[MetronomeAction, MetronomeView], []game.Event, error) {
	switch {
	case s.result != nil || seat != MetronomeSeat:
		return nil, nil, game.ErrNotYourTurn
	case a.Kind == MetronomePause && !s.paused:
		s.paused = true
	case a.Kind == MetronomeResume && s.paused:
		s.paused = false
	case a.Kind == MetronomeTap:
		s.taps++
	default:
		return nil, nil, game.IllegalActionError{Msg: "cannot " + string(a.Kind) + " now"}
	}
	return s, nil, nil
}

func (s metronomeState) NextTick() (time.Duration, bool) {
	return MetronomeInterval, s.result == nil && !s.paused
}

func (s metronomeState) Tick(rng *rand.Rand) (game.State[MetronomeAction, MetronomeView], []game.Event) {
	s.total += 1 + rng.IntN(3)
	s.ticks++
	if s.total >= MetronomeTarget {
		s.result = &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{MetronomeSeat}, Reason: ReasonTargetReached}
	}
	return s, []game.Event{BeatEvent{Total: s.total}}
}

func (s metronomeState) Pause() game.State[MetronomeAction, MetronomeView] {
	s.paused = true
	return s
}

func (s metronomeState) View(game.Seat) MetronomeView {
	return MetronomeView{Total: s.total, Ticks: s.ticks, Taps: s.taps, Paused: s.paused}
}

func (s metronomeState) ToAct() []game.Seat {
	if s.result != nil {
		return []game.Seat{}
	}
	return []game.Seat{MetronomeSeat}
}

func (s metronomeState) Outcome() (game.Result, bool) {
	if s.result == nil {
		return game.Result{}, false
	}
	return *s.result, true
}

func (s metronomeState) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[MetronomeAction, MetronomeView], []game.Event) {
	if s.result == nil {
		s.result = new(game.ForfeitToRemaining([]game.Seat{MetronomeSeat}, seat, reason))
	}
	return s, nil
}
