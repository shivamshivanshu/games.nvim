// Package g2048 is solo 2048: slide and merge tiles on a 4x4 board until no move is left.
package g2048

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

const directionField = "direction"

// Rules implements game.Rules for 2048.
type Rules struct{}

// EventTypes implements game.EventCatalog.
func (Rules) EventTypes() []game.Event { return []game.Event{MovedEvent{}, ReachedEvent{}} }

// Reasons implements game.ReasonCatalog.
func (Rules) Reasons() []game.ResultReason { return []game.ResultReason{ReasonNoMoves} }

// Info describes 2048: solo, no options.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeG2048,
		Name:         "2048",
		Players:      game.PlayerRange{Min: 1, Max: 1},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Actions: []game.ActionSpec{{
			Kind:        ActionMove,
			Description: "Slide every tile one way; two equal tiles that meet merge into their sum.",
			Input: &game.ActionInput{
				Field:       directionField,
				Description: "The way the tiles slide.",
				Values:      grid.DirectionNames(),
			},
		}},
	}
}

// New seats the challenger alone on a board with two tiles spawned from rng.
func (Rules) New(_ Opts, challenger game.UserID, _ []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	first, _ := board{}.spawned(rng)
	second, _ := first.spawned(rng)
	return state{board: second}, game.SoloSeat(challenger), nil
}
