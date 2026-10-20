package checkers

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// TypeCheckers is the game type id.
const TypeCheckers game.GameType = "checkers"

const optionColor game.OptionName = "color"

const textField = "text"

// movePattern matches the move text matchMove reads: a step, or a capture listing every landing.
const movePattern = `^\s*([0-9]{1,2}-[0-9]{1,2}|[0-9]{1,2}(x[0-9]{1,2})+)\s*$`

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
			{
				Name:    optionColor,
				Values:  game.OptionValues(ColorRandom, ColorBlack, ColorWhite),
				Default: game.OptionValue(ColorRandom),
			},
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
	if len(invitees) != 1 {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("checkers needs exactly one opponent, got %d", len(invitees))}
	}
	challengerSeat, opponentSeat := SeatBlack, SeatWhite
	if !game.PickFirst(opts.Color.firstMover(), rng) {
		challengerSeat, opponentSeat = SeatWhite, SeatBlack
	}
	seats := map[game.UserID]game.Seat{challenger: challengerSeat, invitees[0]: opponentSeat}
	return newState(StartBoard(), SeatBlack, 0), seats, nil
}
