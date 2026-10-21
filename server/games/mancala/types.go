package mancala

import "github.com/shivamshivanshu/games.nvim/server/game"

// TypeMancala is the game type on the wire.
const TypeMancala game.GameType = "mancala"

// SeatSouth moves first; SeatNorth second. Each sits behind its own row of pits.
const (
	SeatSouth game.Seat = "south"
	SeatNorth game.Seat = "north"
)

// The ways a game ends once one side's pits are empty: the larger store wins, equal stores draw.
const (
	ReasonMoreSeeds  game.ResultReason = "more_seeds"
	ReasonEqualSeeds game.ResultReason = "equal_seeds"
)

// Opts are the options a mancala game is created with.
type Opts struct {
	First       game.FirstMover  `json:"first"`
	TimeControl game.TimeControl `json:"time_control"`
}

// ActionSow sows the seeds of one of the mover's pits.
const ActionSow game.ActionKind = "sow"

// Action sows Pit, the mover's own pit as text: "1" to "6", counted from the mover's left
// toward its store.
type Action struct {
	Kind game.ActionKind `json:"kind"`
	Pit  string          `json:"pit"`
}

// Side is one seat's row: Pits[0] is its pit 1, on its left, and Pits[5] its pit 6, next to
// its Store.
type Side struct {
	Pits  []int `json:"pits"`
	Store int   `json:"store"`
}

// View holds both sides. LegalPits is the viewer's own: the pits it may sow, empty unless it is
// to move.
type View struct {
	South     Side  `json:"south"`
	North     Side  `json:"north"`
	LegalPits []int `json:"legal_pits"`
}

// SowedEvent reports a sowing: Captured counts the seeds it took into By's store, and Again is
// true when By moves again, its last seed having fallen in that store.
type SowedEvent struct {
	By       game.Seat `json:"by"`
	Pit      int       `json:"pit"`
	Captured int       `json:"captured"`
	Again    bool      `json:"again"`
}

// EventSowed is the kind of SowedEvent.
const EventSowed game.EventKind = "sowed"

// EventKind implements game.Event.
func (SowedEvent) EventKind() game.EventKind { return EventSowed }

// CollectedEvent reports the seeds left in By's pits at the end, moved to By's store.
type CollectedEvent struct {
	By    game.Seat `json:"by"`
	Seeds int       `json:"seeds"`
}

// EventCollected is the kind of CollectedEvent.
const EventCollected game.EventKind = "collected"

// EventKind implements game.Event.
func (CollectedEvent) EventKind() game.EventKind { return EventCollected }
