package chess

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const TypeChess game.GameType = "chess"

const optionColor game.OptionName = "color"

const moveField = "move"

type Rules struct{}

func (Rules) EventTypes() []game.Event { return []game.Event{MovedEvent{}} }

func (Rules) Reasons() []game.ResultReason {
	return []game.ResultReason{
		ReasonCheckmate, ReasonStalemate, ReasonInsufficientMaterial, ReasonFivefold,
		ReasonSeventyFive, ReasonThreefold, ReasonFifty,
	}
}

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
		Actions: []game.ActionSpec{
			{
				Kind:        ActionMove,
				Description: "Play a move.",
				Input: &game.ActionInput{
					Field:       moveField,
					Description: "A move in SAN or UCI, any case, e.g. e4, Nf3, O-O, exd8=Q, e2e4.",
					Pattern:     MovePattern,
				},
			},
			{
				Kind:        ActionClaimDraw,
				Description: "Claim a draw by threefold repetition or the fifty-move rule, on your move.",
			},
		},
	}
}

func (Rules) New(opts Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	start, err := ParseFEN(StartFEN)
	if err != nil {
		return nil, nil, err
	}
	return NewGame(start, opts, challenger, invitees, rng)
}

// NewGame is a game of chess from start, the challenger's colour picked by opts.
func NewGame(start Position, opts Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	if len(invitees) != 1 {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("chess needs exactly one opponent, got %d", len(invitees))}
	}
	challengerSeat, opponentSeat := SeatWhite, SeatBlack
	if !game.PickFirst(opts.Color.firstMover(), rng) {
		challengerSeat, opponentSeat = SeatBlack, SeatWhite
	}
	seats := map[game.UserID]game.Seat{challenger: challengerSeat, invitees[0]: opponentSeat}
	return newState(start), seats, nil
}
