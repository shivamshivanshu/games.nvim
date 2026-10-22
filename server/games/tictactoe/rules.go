package tictactoe

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const cellField = "cell"

// Rules is Tic-Tac-Toe: two players, three in a row on a 3x3 board.
type Rules struct{}

// Info implements game.Rules.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeTicTacToe,
		Name:         "Tic-Tac-Toe",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Options:      []game.OptionSpec{game.FirstMoverOption()},
		Actions: []game.ActionSpec{{
			Kind:        ActionPlace,
			Description: "Place your mark on an empty cell.",
			Input: &game.ActionInput{
				Field:       cellField,
				Description: "The cell, numbered 1-9 in phone layout: 1 top-left, 9 bottom-right.",
				Values:      game.NumberedValues(cellCount),
			},
		}},
		Events:  []game.Event{PlacedEvent{}},
		Reasons: []game.ResultReason{ReasonThreeInARow, ReasonBoardFull},
	}
}

// New implements game.Rules. The first mover, picked by Opts.First, plays X.
func (Rules) New(opts Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	return newState(), sides.Assign(opts.First, challenger, invitees, rng), nil
}
