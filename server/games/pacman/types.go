package pacman

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// TypePacman is the game type on the wire.
const TypePacman game.GameType = "pacman"

// SeatPlayer is the only seat.
const SeatPlayer game.Seat = "player"

// The game's own result reasons.
const (
	// ReasonGameOver ends a game whose last life was lost: a loss.
	ReasonGameOver game.ResultReason = "game_over"
	// ReasonCleared ends a game whose last level was cleared: a win.
	ReasonCleared game.ResultReason = "cleared"
)

// Opts is empty: Pac-Man has no options.
type Opts struct{}

// The actions.
const (
	// ActionTurn turns Pac-Man Direction at the first cell where that way is open, and resumes a paused game.
	ActionTurn game.ActionKind = "turn"
	// ActionPause stops the game until the next turn.
	ActionPause game.ActionKind = "pause"
)

// Action is a turn, with Direction one of grid.DirectionNames, or a pause, without one.
type Action struct {
	Kind      game.ActionKind `json:"kind"`
	Direction string          `json:"direction"`
}

// GhostName names a ghost, and with it how it hunts.
type GhostName string

// The ghosts, in the order View.Ghosts lists them.
const (
	Blinky GhostName = "blinky"
	Pinky  GhostName = "pinky"
	Inky   GhostName = "inky"
	Clyde  GhostName = "clyde"
)

// GhostMode is what a ghost is doing.
type GhostMode string

// The ghost modes.
const (
	// ModeHouse is waiting in the ghost house, or on the way out of it.
	ModeHouse GhostMode = "house"
	// ModeScatter is heading for the ghost's own corner.
	ModeScatter GhostMode = "scatter"
	// ModeChase is hunting Pac-Man, each ghost its own way.
	ModeChase GhostMode = "chase"
	// ModeFrightened is wandering at random after a power pellet; Pac-Man can eat it.
	ModeFrightened GhostMode = "frightened"
	// ModeEyes is an eaten ghost's eyes going back to the house.
	ModeEyes GhostMode = "eyes"
)

// Actor is where Pac-Man or a ghost is: Row and Col are 1-based from the top left.
type Actor struct {
	Row    int            `json:"row"`
	Col    int            `json:"col"`
	Facing grid.Direction `json:"facing"`
}

// PacmanView is Pac-Man, and the turn it will take at the first cell where it can; null for none.
type PacmanView struct {
	Actor
	NextTurn *grid.Direction `json:"next_turn" jsonschema:"nullable"`
}

// GhostView is one ghost.
type GhostView struct {
	Actor
	Name GhostName `json:"name"`
	Mode GhostMode `json:"mode"`
}

// View is the whole game. Maze has one line per row, top first, of one Tile letter per cell. Lives
// counts the life in play. FrightTicks is how many ticks the ghosts stay frightened. CanTurn and
// CanPause tell the player which actions are open: both are false once the game is over, and
// CanPause is false while paused.
type View struct {
	Maze        []string    `json:"maze"`
	Pacman      PacmanView  `json:"pacman"`
	Ghosts      []GhostView `json:"ghosts"`
	Score       int         `json:"score"`
	Lives       int         `json:"lives"`
	Level       int         `json:"level"`
	Levels      int         `json:"levels"`
	FoodLeft    int         `json:"food_left"`
	FrightTicks int         `json:"fright_ticks"`
	Paused      bool        `json:"paused"`
	CanTurn     bool        `json:"can_turn"`
	CanPause    bool        `json:"can_pause"`
}

// GhostEatenEvent reports Pac-Man eating a frightened ghost, and the points it scored.
type GhostEatenEvent struct {
	By     game.Seat `json:"by"`
	Ghost  GhostName `json:"ghost"`
	Points int       `json:"points"`
}

// EventGhostEaten is the kind of GhostEatenEvent.
const EventGhostEaten game.EventKind = "ghost_eaten"

// EventKind implements game.Event.
func (GhostEatenEvent) EventKind() game.EventKind { return EventGhostEaten }

// CaughtEvent reports a ghost catching Pac-Man, and the lives left after it.
type CaughtEvent struct {
	By        game.Seat `json:"by"`
	Ghost     GhostName `json:"ghost"`
	LivesLeft int       `json:"lives_left"`
}

// EventCaught is the kind of CaughtEvent.
const EventCaught game.EventKind = "caught"

// EventKind implements game.Event.
func (CaughtEvent) EventKind() game.EventKind { return EventCaught }

// LevelClearedEvent reports Pac-Man eating the last dot of a level.
type LevelClearedEvent struct {
	By    game.Seat `json:"by"`
	Level int       `json:"level"`
}

// EventLevelCleared is the kind of LevelClearedEvent.
const EventLevelCleared game.EventKind = "level_cleared"

// EventKind implements game.Event.
func (LevelClearedEvent) EventKind() game.EventKind { return EventLevelCleared }
