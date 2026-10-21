// Package mancala is Kalah(6,4): two rows of six pits with four seeds each, and a store per side.
package mancala

import (
	"fmt"
	"math/rand/v2"
	"strconv"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const pitField = "pit"

// Rules implements game.Rules for mancala.
type Rules struct{}

// EventTypes implements game.EventCatalog.
func (Rules) EventTypes() []game.Event { return []game.Event{SowedEvent{}, CollectedEvent{}} }

// Reasons implements game.ReasonCatalog.
func (Rules) Reasons() []game.ResultReason {
	return []game.ResultReason{ReasonMoreSeeds, ReasonEqualSeeds}
}

// Info describes mancala to the catalogue.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeMancala,
		Name:         "Mancala",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapDraw, game.CapClock, game.CapAbort},
		Options:      []game.OptionSpec{game.FirstMoverOption(), game.TimeControlOption()},
		Actions: []game.ActionSpec{{
			Kind:        ActionSow,
			Description: "Sow every seed of one of your pits, one per pit counter-clockwise, into your store but not your opponent's.",
			Input: &game.ActionInput{
				Field:       pitField,
				Description: "Your pit, 1 to 6 from your left toward your store.",
				Values:      pitNames(),
			},
		}},
	}
}

func pitNames() []string {
	names := make([]string, pitsPerSide)
	for i := range names {
		names[i] = strconv.Itoa(i + 1)
	}
	return names
}

// parsePit is the 1-based pit text names, which must be one of pitNames.
func parsePit(text string) (int, error) {
	for i, name := range pitNames() {
		if text == name {
			return i + 1, nil
		}
	}
	return 0, game.IllegalActionError{Msg: fmt.Sprintf("pit must be 1 to %d, got %q", pitsPerSide, text)}
}

// New seats the first mover as south, which sows first.
func (Rules) New(opts Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	seats := sides.Assign(challenger, invitees[0], game.PickFirst(opts.First, rng))
	return state{board: startBoard(), toMove: SeatSouth}, seats, nil
}
