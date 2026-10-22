// Package mancala is Kalah(6,4): two rows of six pits with four seeds each, and a store per side.
package mancala

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const pitField = "pit"

// Rules implements game.Rules for mancala.
type Rules struct{}

// Info describes mancala to the catalogue.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeMancala,
		Name:         "Mancala",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapDraw, game.CapClock, game.CapAbort},
		Options:      game.TurnOptions(),
		Actions: []game.ActionSpec{{
			Kind:        ActionSow,
			Description: "Sow every seed of one of your pits, one per pit counter-clockwise, into your store but not your opponent's.",
			Input: &game.ActionInput{
				Field:       pitField,
				Description: "Your pit, 1 to 6 from your left toward your store.",
				Values:      game.NumberedValues(pitsPerSide),
			},
		}},
		Events:  []game.Event{SowedEvent{}, CollectedEvent{}},
		Reasons: []game.ResultReason{ReasonMoreSeeds, ReasonEqualSeeds},
	}
}

func parsePit(text string) (int, error) {
	pit, ok := game.ParseNumbered(text, pitsPerSide)
	if !ok {
		return 0, game.Illegalf("pit must be 1 to %d, got %q", pitsPerSide, text)
	}
	return pit, nil
}

// New seats the first mover as south, which sows first.
func (Rules) New(opts game.TurnOpts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	return state{Turn: game.TurnTo(SeatSouth), board: startBoard()}, sides.AssignFirst(opts.First, challenger, invitees, rng), nil
}
