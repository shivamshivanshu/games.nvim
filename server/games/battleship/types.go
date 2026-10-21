package battleship

import "github.com/shivamshivanshu/games.nvim/server/game"

// TypeBattleship is the game's catalogue id.
const TypeBattleship game.GameType = "battleship"

var sides = game.Pair{game.NumberedSeat(0), game.NumberedSeat(1)}

// Opts are the options a game is created with.
type Opts struct {
	First game.FirstMover `json:"first"`
}

// The actions. A place puts the viewer's next ship with its left or top end on
// Cell; random places the rest of the fleet; fire shoots at Cell.
const (
	ActionPlaceAcross game.ActionKind = "place_across"
	ActionPlaceDown   game.ActionKind = "place_down"
	ActionRandom      game.ActionKind = "random"
	ActionFire        game.ActionKind = "fire"
)

// Action names its cell as text, such as "c5": empty for random.
type Action struct {
	Kind game.ActionKind `json:"kind"`
	Cell string          `json:"cell,omitempty"`
}

// Phase is where the game is: placing fleets, shooting, or finished.
type Phase string

// The phases.
const (
	PhaseSetup  Phase = "setup"
	PhaseBattle Phase = "battle"
	PhaseOver   Phase = "over"
)

// Tile is what one square of a board shows its viewer. On the viewer's own
// board: water, a ship, or the opponent's miss, hit or sunk ship. On the target
// board: a square not fired at, or the viewer's miss, hit or sunk ship; once
// the game is over the opponent's ships that were never hit show as TileShip.
type Tile string

// The tiles.
const (
	TileEmpty Tile = "empty"
	TileShip  Tile = "ship"
	TileMiss  Tile = "miss"
	TileHit   Tile = "hit"
	TileSunk  Tile = "sunk"
)

// Square is a board position on the wire, 1-based: row 1 is the top, column 1 the left.
type Square struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// ShipSpec is a ship of the fleet before it is placed.
type ShipSpec struct {
	Name ShipName `json:"name"`
	Size int      `json:"size"`
}

// ShipState is how a ship stands: not yet placed, afloat, or sunk. The
// opponent's ships are only ever afloat or sunk to the viewer.
type ShipState string

// The ship states.
const (
	ShipUnplaced ShipState = "unplaced"
	ShipAfloat   ShipState = "afloat"
	ShipSunk     ShipState = "sunk"
)

// ShipStatus is one ship of a fleet as the viewer may know it.
type ShipStatus struct {
	Name  ShipName  `json:"name"`
	Size  int       `json:"size"`
	State ShipState `json:"state"`
}

// Anchors are the squares the viewer's next ship may start on, its left end
// lying across or its top end lying down. Both are empty unless the viewer is placing.
type Anchors struct {
	Across []Square `json:"across"`
	Down   []Square `json:"down"`
}

// View holds both boards as rows from the top: Own is the viewer's waters,
// Target the opponent's as the viewer's shots found them. Next and Anchors are
// the viewer's own placement; Targets the squares it may fire at, none unless
// it is to fire. Last is the latest shot of either seat.
type View struct {
	Phase   Phase        `json:"phase"`
	Own     [][]Tile     `json:"own"`
	Target  [][]Tile     `json:"target"`
	Fleet   []ShipStatus `json:"fleet"`
	Enemy   []ShipStatus `json:"enemy"`
	Next    *ShipSpec    `json:"next" jsonschema:"nullable"`
	Anchors Anchors      `json:"anchors"`
	Targets []Square     `json:"targets"`
	Last    *ShotEvent   `json:"last" jsonschema:"nullable"`
}

// ShotOutcome is what a shot found: water, a ship, or the last afloat square of a ship.
type ShotOutcome string

// The shot outcomes.
const (
	ShotMiss ShotOutcome = "miss"
	ShotHit  ShotOutcome = "hit"
	ShotSunk ShotOutcome = "sunk"
)

// ReadyEvent reports that By has placed its whole fleet.
type ReadyEvent struct {
	By game.Seat `json:"by"`
}

// ShotEvent reports By's shot at Cell of the opponent's waters; Ship names the
// ship it sank, and is empty otherwise.
type ShotEvent struct {
	By      game.Seat   `json:"by"`
	Cell    Square      `json:"cell"`
	Outcome ShotOutcome `json:"outcome"`
	Ship    ShipName    `json:"ship,omitempty"`
}

// The kinds of ReadyEvent and ShotEvent.
const (
	EventReady game.EventKind = "ready"
	EventShot  game.EventKind = "shot"
)

// EventKind implements game.Event.
func (ReadyEvent) EventKind() game.EventKind { return EventReady }

// EventKind implements game.Event.
func (ShotEvent) EventKind() game.EventKind { return EventShot }

// ReasonFleetSunk ends a game won by sinking the opponent's whole fleet.
const ReasonFleetSunk game.ResultReason = "fleet_sunk"
