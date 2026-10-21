package reversi

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/discs"
)

const boardSize = 8

// Rules is Reversi on 8x8: a move must flip a disc, a seat with none passes,
// and the game ends when neither seat can move.
type Rules struct{}

// Info implements game.Rules.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeReversi,
		Name:         "Reversi",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapDraw, game.CapClock, game.CapAbort},
		Options:      discs.Options(),
		Actions: []game.ActionSpec{
			discs.PlaceSpec(discs.NewBoard(boardSize).Coords(), "Place a disc that flips at least one of the opponent's."),
		},
		Events:  []game.Event{PlacedEvent{}, PassedEvent{}},
		Reasons: []game.ResultReason{ReasonMoreDiscs, ReasonEqualDiscs},
	}
}

// New implements game.Rules. The first mover, picked by Opts.First, plays black.
func (Rules) New(opts discs.Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[discs.Action, View], map[game.UserID]game.Seat, error) {
	seats := discs.Sides.Assign(challenger, invitees[0], game.PickFirst(opts.First, rng))
	return state{Turn: discs.FirstTurn(), board: startBoard()}, seats, nil
}
