package chess

import (
	"fmt"
	"math/rand/v2"

	"games.nvim/server/game"
)

type Rules struct{}

func (Rules) Info() game.Info {
	return game.Info{
		Type:         "chess",
		Name:         "Chess",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapDraw, game.CapClock, game.CapAbort},
		Options: []game.OptionSpec{
			{
				Name:    game.OptionTimeControl,
				Values:  []string{string(TimeControlNone), string(TimeControl3Plus2), string(TimeControl5Plus0), string(TimeControl10Plus5), string(TimeControl15Plus10)},
				Default: string(TimeControl10Plus5),
			},
			{
				Name:    "color",
				Values:  []string{string(ColorRandom), string(ColorWhite), string(ColorBlack)},
				Default: string(ColorRandom),
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
	if opts.challengerPlaysBlack(rng) {
		challengerSeat, opponentSeat = SeatBlack, SeatWhite
	}
	seats := map[game.UserID]game.Seat{challenger: challengerSeat, invitees[0]: opponentSeat}
	return newState(start), seats, nil
}

func (o Opts) challengerPlaysBlack(rng *rand.Rand) bool {
	switch o.Color {
	case ColorWhite:
		return false
	case ColorBlack:
		return true
	}
	return rng.IntN(2) == 1
}
