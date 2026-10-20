package tictactoe

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type Rules struct{}

func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeTicTacToe,
		Name:         "Tic-Tac-Toe",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Options:      []game.OptionSpec{game.FirstMoverOption()},
	}
}

func (Rules) New(opts Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	if len(invitees) != 1 {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("tictactoe needs exactly one opponent, got %d", len(invitees))}
	}
	challengerSeat, opponentSeat := SeatX, SeatO
	if !game.PickFirst(opts.First, rng) {
		challengerSeat, opponentSeat = SeatO, SeatX
	}
	seats := map[game.UserID]game.Seat{challenger: challengerSeat, invitees[0]: opponentSeat}
	return newState(), seats, nil
}
