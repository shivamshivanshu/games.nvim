package checkers

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// TypeCheckers is the game type id.
const TypeCheckers game.GameType = "checkers"

const textField = "text"

// Rules is American/English checkers on an 8x8 board.
type Rules struct{}

// EventTypes lists the events checkers emits, for the wire schema.
func (Rules) EventTypes() []game.Event { return []game.Event{MovedEvent{}} }

// Reasons lists the result reasons checkers adds to the core's.
func (Rules) Reasons() []game.ResultReason {
	return []game.ResultReason{ReasonNoPieces, ReasonNoMoves, ReasonFortyMove}
}

// Info describes checkers: two players, colour and time-control options.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeCheckers,
		Name:         "Checkers",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapDraw, game.CapClock, game.CapAbort},
		Options: []game.OptionSpec{
			game.TimeControlOption(),
			game.SideOption(ColorBlack, ColorWhite),
		},
		Actions: []game.ActionSpec{{
			Kind:        ActionMove,
			Description: "Play one complete move; a capture lists every landing square.",
			Input: &game.ActionInput{
				Field:       textField,
				Description: "Squares 1-32, e.g. 11-15, 15x24 or 15x24x31.",
				Pattern:     movePattern,
			},
		}},
	}
}

// New seats the challenger by the colour option and starts with Black to move.
func (Rules) New(opts Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	challengerFirst := game.PickFirst(game.SideFirstMover(opts.Color, ColorBlack, ColorWhite), rng)
	return newState(StartBoard(), SeatBlack, 0), sides.Assign(challenger, invitees[0], challengerFirst), nil
}
