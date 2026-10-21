// Package pacman is solo Pac-Man in real time: the server ticks the game, and the player turns
// Pac-Man through the arcade maze, eating every dot over three levels while four ghosts hunt it.
package pacman

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

const directionField = "direction"

// Rules implements game.Rules for Pac-Man.
type Rules struct{}

// EventTypes implements game.EventCatalog.
func (Rules) EventTypes() []game.Event {
	return []game.Event{GhostEatenEvent{}, CaughtEvent{}, LevelClearedEvent{}}
}

// Reasons implements game.ReasonCatalog.
func (Rules) Reasons() []game.ResultReason { return []game.ResultReason{ReasonGameOver, ReasonCleared} }

// Info describes Pac-Man: solo, no options.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypePacman,
		Name:         "Pac-Man",
		Players:      game.PlayerRange{Min: 1, Max: 1},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Actions: []game.ActionSpec{
			{
				Kind:        ActionTurn,
				Description: "Turn Pac-Man at the first cell where that way is open; resumes a paused game.",
				Input: &game.ActionInput{
					Field:       directionField,
					Description: "The way to turn.",
					Values:      grid.DirectionNames(),
				},
			},
			{Kind: ActionPause, Description: "Stop the game until the next turn."},
		},
	}
}

// New seats the challenger alone at level 1, paused until the first turn.
func (Rules) New(_ Opts, challenger game.UserID, invitees []game.UserID, _ *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	if len(invitees) != 0 {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("pacman is solo, got %d opponents", len(invitees))}
	}
	return newState(), map[game.UserID]game.Seat{challenger: SeatPlayer}, nil
}
