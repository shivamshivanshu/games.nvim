package game

import (
	"encoding/json"
	"math/rand/v2"
	"reflect"
	"time"
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
	// Conclude takes a result the core imposed, such as an agreed draw or an
	// abort, so that the state stops offering moves: ToAct is empty and the
	// views show none.
	Conclude(result Result) State[Action, View]
}

// Scorer is optionally implemented by a finished state to report each seat's
// score, such as a chip delta, in place of the core's win/draw/loss default.
type Scorer interface {
	Scores() map[Seat]int64
}

// ExportContext is what the core knows of a finished game beyond its state:
// Result is the final one, including a result the core imposed.
type ExportContext struct {
	Players   []Player
	StartedAt time.Time
	Result    Result
}

// Exporter is optionally implemented by a finished state to give files that
// record the game, such as a PGN, by file name. The files are text.
type Exporter interface {
	Exports(ctx ExportContext) map[string][]byte
}

// Opener is optionally implemented by a state fresh from New to give the
// events of reaching it, such as moves the rules play before anyone acts.
type Opener interface {
	OpeningEvents() []Event
}

// Ticking is optionally implemented by a state that moves on its own, such as
// a real-time game. The core ticks it as a step of its own, recorded like an
// action, so a restore replays it exactly; the rules read no clock.
type Ticking[Action, View any] interface {
	// NextTick is how long after reaching this state its next tick is due,
	// and false while it waits on its players, such as paused or finished.
	NextTick() (time.Duration, bool)
	// Tick advances the state by one tick, drawing from the tick step's rng.
	Tick(rng *rand.Rand) (State[Action, View], []Event)
	// Pause is the state stopped until its player acts again; the core
	// pauses a game whose player went away.
	Pause() State[Action, View]
}

// Knockout is optionally implemented by a state whose rules put seats out of
// play before the game ends, such as a player left with no dice or no chips.
// The core counts those seats as eliminated without calling Eliminate: they
// get no controls, and an ending the core imposes still records their loss.
type Knockout interface {
	KnockedOut() []Seat
}

// Game is the type-erased side the server core uses.
type Game interface {
	Info() Info
	ResolveOptions(raw json.RawMessage) (Options, error)
	New(opts Options, challenger UserID, invitees []UserID, rng *rand.Rand) (ErasedState, map[UserID]Seat, error)
	// WireTypes are the Go types of the game's action and view.
	WireTypes() (action, view reflect.Type)
}

type ErasedState interface {
	Apply(seat Seat, action json.RawMessage, rng *rand.Rand) (ErasedState, []json.RawMessage, error)
	View(seat Seat) (json.RawMessage, error)
	ToAct() []Seat
	Outcome() (Result, bool)
	Eliminate(seat Seat, reason ResultReason) (ErasedState, []json.RawMessage, error)
	Scores() (map[Seat]int64, bool)
	// Exports is empty for a state that is not an Exporter.
	Exports(ctx ExportContext) map[string][]byte
	Conclude(result Result) ErasedState
	// OpeningEvents are the encoded events of an Opener fresh from New, and
	// empty otherwise.
	OpeningEvents() []json.RawMessage
	// KnockedOut are the seats a Knockout state's rules put out, and empty
	// otherwise.
	KnockedOut() []Seat
	// RealTime is whether the state is Ticking, even while no tick is due.
	RealTime() bool
	// NextTick is a Ticking state's own, and false for any other.
	NextTick() (time.Duration, bool)
	// Tick fails for a state that is not Ticking.
	Tick(rng *rand.Rand) (ErasedState, []json.RawMessage, error)
	// Pause is the state itself for a state that is not Ticking.
	Pause() ErasedState
}
