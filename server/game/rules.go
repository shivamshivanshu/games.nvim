package game

import (
	"encoding/json"
	"math/rand/v2"
	"slices"
)

// Rules is what a game author implements, with concrete types and no JSON.
type Rules[Opts, Action, View any] interface {
	Info() Info
	New(opts Opts, challenger UserID, invitees []UserID, rng *rand.Rand) (State[Action, View], map[UserID]Seat, error)
}

type State[Action, View any] interface {
	Apply(seat Seat, action Action, rng *rand.Rand) (State[Action, View], []Event, error)
	View(seat Seat) View
	ToAct() []Seat
	Outcome() (Result, bool)
	Eliminate(seat Seat, reason ResultReason) (State[Action, View], []Event)
}

// Game is the type-erased side the server core uses.
type Game interface {
	Info() Info
	ResolveOptions(raw json.RawMessage) (Options, error)
	New(opts Options, challenger UserID, invitees []UserID, rng *rand.Rand) (ErasedState, map[UserID]Seat, error)
}

type ErasedState interface {
	Apply(seat Seat, action json.RawMessage, rng *rand.Rand) (ErasedState, []Event, error)
	View(seat Seat) (json.RawMessage, error)
	ToAct() []Seat
	Outcome() (Result, bool)
	Eliminate(seat Seat, reason ResultReason) (ErasedState, []Event)
}

// ForfeitToRemaining makes every seat except the eliminated one a winner.
func ForfeitToRemaining(all []Seat, eliminated Seat, reason ResultReason) Result {
	winners := slices.DeleteFunc(slices.Clone(all), func(s Seat) bool { return s == eliminated })
	return Result{Kind: ResultDecisive, Winners: winners, Reason: reason}
}
