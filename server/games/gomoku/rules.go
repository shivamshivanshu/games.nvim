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

// EventTypes implements game.EventCatalog.
func (Rules) EventTypes() []game.Event { return []game.Event{PlacedEvent{}} }

// Reasons implements game.ReasonCatalog.
func (Rules) Reasons() []game.ResultReason {
	return []game.ResultReason{ReasonFiveInARow, ReasonBoardFull}
}

// Info implements game.Rules.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeGomoku,
		Name:         "Gomoku",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapDraw, game.CapClock, game.CapAbort},
		Options:      discs.Options(),
		Actions: []game.ActionSpec{
			discs.PlaceSpec(discs.NewBoard(boardSize).Coords(), "Place a stone on an empty cell."),
		},
	}
}

// New implements game.Rules. The first mover, picked by Opts.First, plays black.
func (Rules) New(opts discs.Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[discs.Action, View], map[game.UserID]game.Seat, error) {
	seats := discs.Sides.Assign(challenger, invitees[0], game.PickFirst(opts.First, rng))
	return newState(), seats, nil
}
