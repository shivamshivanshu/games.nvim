package gomoku

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/discs"
)

const (
	boardSize = 15
	winLength = 5
)

// Rules is freestyle Gomoku: two players on 15x15, five or more in a row wins.
type Rules struct{}

// Info implements game.Rules.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeGomoku,
		Name:         "Gomoku",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapDraw, game.CapClock, game.CapAbort},
		Options:      game.TurnOptions(),
		Actions: []game.ActionSpec{
			discs.PlaceSpec(discs.NewBoard(boardSize).Coords(), "Place a stone on an empty cell."),
		},
		Events:  []game.Event{PlacedEvent{}},
		Reasons: []game.ResultReason{ReasonFiveInARow, ReasonBoardFull},
	}
}

// New implements game.Rules. The first mover, picked by Opts.First, plays black.
func (Rules) New(opts game.TurnOpts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[discs.Action, View], map[game.UserID]game.Seat, error) {
	return newState(), discs.Sides.AssignFirst(opts.First, challenger, invitees, rng), nil
}
