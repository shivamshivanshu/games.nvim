// Package pacman is solo Pac-Man in real time: the server ticks the game, and the player turns
// Pac-Man through the arcade maze, eating every dot over three levels while four ghosts hunt it.
package pacman

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

const directionField = "direction"

// Rules implements game.Rules for Pac-Man.
type Rules struct{}

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
		Events:  []game.Event{GhostEatenEvent{}, CaughtEvent{}, LevelClearedEvent{}},
		Reasons: []game.ResultReason{ReasonGameOver, ReasonCleared},
	}
}

// New seats the challenger alone at level 1, paused until the first turn.
func (Rules) New(_ Opts, challenger game.UserID, _ []game.UserID, _ *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	return newState(), game.SoloSeat(challenger), nil
}
