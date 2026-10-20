package chess

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const TypeChess game.GameType = "chess"

const optionColor game.OptionName = "color"

type Rules struct{}

func optionValues[T ~string](vs ...T) []game.OptionValue {
	out := make([]game.OptionValue, len(vs))
	for i, v := range vs {
		out[i] = game.OptionValue(v)
	}
	return out
}

func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeChess,
		Name:         "Chess",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapDraw, game.CapClock, game.CapAbort},
		Options: []game.OptionSpec{
			{
				Name:    game.OptionTimeControl,
				Values:  optionValues(TimeControlNone, TimeControl3Plus2, TimeControl5Plus0, TimeControl10Plus5, TimeControl15Plus10),
				Default: game.OptionValue(TimeControl10Plus5),
			},
			{
				Name:    optionColor,
				Values:  optionValues(ColorRandom, ColorWhite, ColorBlack),
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
