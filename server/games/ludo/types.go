package ludo

import "github.com/shivamshivanshu/games.nvim/server/game"

// Opts are Ludo's resolved options.
type Opts struct {
	TimeControl game.TimeControl `json:"time_control"`
}

// The seats, named after the shared seat-colour order.
const (
	SeatBlue   game.Seat = "blue"
	SeatYellow game.Seat = "yellow"
	SeatRed    game.Seat = "red"
	SeatGreen  game.Seat = "green"
)

// Progress is a token's distance along its own route: ProgressBase, then
// ProgressStart..lastTrackProgress on the shared track, the home column, and ProgressHome.
type Progress int

// Square is a shared-track square, numbered 0 to TrackLength-1 clockwise.
type Square int

// LocationKind is the part of the board a token stands on.
type LocationKind string

// The location kinds.
const (
	LocationBase       LocationKind = "base"
	LocationTrack      LocationKind = "track"
	LocationHomeColumn LocationKind = "home_column"
	LocationHome       LocationKind = "home"
)

// Location is where a token is drawn: Square on the track, or Step 1 to 5 up its seat's home column.
type Location struct {
	Kind   LocationKind `json:"kind"`
	Square *Square      `json:"square,omitempty"`
	Step   *int         `json:"step,omitempty"`
}

// Token numbers a seat's tokens from 1 to TokensPerSeat.
type Token int

// Roll is the face of one die throw.
type Roll int

// Phase is what the seat to act must do next.
type Phase string

// The phases of a turn.
const (
	PhaseRoll     Phase = "roll"
	PhaseMove     Phase = "move"
	PhaseFinished Phase = "finished"
)

// The action kinds.
const (
	ActionRoll game.ActionKind = "roll"
	ActionMove game.ActionKind = "move"
)

// Action is a roll, or a move of the token numbered Token by the last roll.
type Action struct {
	Kind  game.ActionKind `json:"kind"`
	Token string          `json:"token,omitempty"`
}

// Tokens is one seat's token progress, indexed by Token-1.
type Tokens [TokensPerSeat]Progress

// Locations is where one seat's tokens are drawn, indexed by Token-1.
type Locations [TokensPerSeat]Location

// View is the board as one seat sees it. Starts maps each seat to its start
// square on the track, and Safe lists the squares where no capture happens;
// Movable and CanRoll are the viewer's own, empty and false unless it is the
// viewer's turn.
type View struct {
	Starts     map[game.Seat]Square    `json:"starts"`
	Safe       []Square                `json:"safe"`
	Progress   map[game.Seat]Tokens    `json:"progress"`
	Locations  map[game.Seat]Locations `json:"locations"`
	LastRoll   Roll                    `json:"last_roll"`
	SixesInRow int                     `json:"sixes_in_row"`
	Phase      Phase                   `json:"phase"`
	Movable    []Token                 `json:"movable"`
	CanRoll    bool                    `json:"can_roll"`
}

// ReasonAllHome ends the game when a seat brings its last token home.
const ReasonAllHome game.ResultReason = "all_home"

// The event kinds.
const (
	EventRolled    game.EventKind = "rolled"
	EventPassed    game.EventKind = "passed"
	EventForfeited game.EventKind = "forfeited"
	EventMoved     game.EventKind = "moved"
	EventCaptured  game.EventKind = "captured"
)

// RolledEvent reports a die throw.
type RolledEvent struct {
	By    game.Seat `json:"by"`
	Value Roll      `json:"value"`
}

// PassedEvent means the roll left no token able to move.
type PassedEvent struct {
	By game.Seat `json:"by"`
}

// ForfeitedEvent means a third six in a row ended the turn.
type ForfeitedEvent struct {
	By game.Seat `json:"by"`
}

// MovedEvent reports a token moving along its route.
type MovedEvent struct {
	By    game.Seat `json:"by"`
	Token Token     `json:"token"`
	From  Location  `json:"from"`
	To    Location  `json:"to"`
}

// CapturedEvent reports a victim's token sent back to base by the mover.
type CapturedEvent struct {
	By     game.Seat `json:"by"`
	Victim game.Seat `json:"victim"`
	Token  Token     `json:"token"`
}

// EventKind is EventRolled.
func (RolledEvent) EventKind() game.EventKind { return EventRolled }

// EventKind is EventPassed.
func (PassedEvent) EventKind() game.EventKind { return EventPassed }

// EventKind is EventForfeited.
func (ForfeitedEvent) EventKind() game.EventKind { return EventForfeited }

// EventKind is EventMoved.
func (MovedEvent) EventKind() game.EventKind { return EventMoved }

// EventKind is EventCaptured.
func (CapturedEvent) EventKind() game.EventKind { return EventCaptured }
