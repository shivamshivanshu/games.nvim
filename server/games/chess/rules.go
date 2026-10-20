package chess

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const TypeChess game.GameType = "chess"

const optionColor game.OptionName = "color"

type Rules struct{}

func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeChess,
		Name:         "Chess",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapDraw, game.CapClock, game.CapAbort},
		Options: []game.OptionSpec{
			game.TimeControlOption(),
			{
				Name:    optionColor,
				Values:  game.OptionValues(ColorRandom, ColorWhite, ColorBlack),
				Default: game.OptionValue(ColorRandom),
			},
		},
	}
}

func (Rules) New(opts Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	if len(invitees) != 1 {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("chess needs exactly one opponent, got %d", len(invitees))}
	}
	start, err := ParseFEN(StartFEN)
	if err != nil {
		return nil, nil, err
	}
	challengerSeat, opponentSeat := SeatWhite, SeatBlack
	if !game.PickFirst(opts.Color.firstMover(), rng) {
		challengerSeat, opponentSeat = SeatBlack, SeatWhite
	}
	seats := map[game.UserID]game.Seat{challenger: challengerSeat, invitees[0]: opponentSeat}
	return newState(start), seats, nil
}
