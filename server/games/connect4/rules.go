package connect4

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const TypeConnect4 game.GameType = "connect4"

type Rules struct{}

func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeConnect4,
		Name:         "Connect4",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapDraw, game.CapClock, game.CapAbort},
		Options:      []game.OptionSpec{game.FirstMoverOption(), game.TimeControlOption()},
	}
}

// New seats the first mover as red.
func (Rules) New(opts Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	if len(invitees) != 1 {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("connect4 needs exactly one opponent, got %d", len(invitees))}
	}
	first, second := challenger, invitees[0]
	if !game.PickFirst(opts.First, rng) {
		first, second = second, first
	}
	seats := map[game.UserID]game.Seat{first: SeatRed, second: SeatYellow}
	return state{toMove: SeatRed}, seats, nil
}
