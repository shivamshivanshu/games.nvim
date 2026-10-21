package connect4

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// TypeConnect4 is the game type of connect4.
const TypeConnect4 game.GameType = "connect4"

// Rules is the connect4 game: 7 columns by 6 rows, four in a row wins.
type Rules struct{}

// Info describes connect4 to the catalogue.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeConnect4,
		Name:         "Connect4",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapDraw, game.CapClock, game.CapAbort},
		Options:      []game.OptionSpec{game.FirstMoverOption(), game.TimeControlOption()},
		Actions: []game.ActionSpec{
			{
				Kind:        ActionDrop,
				Description: "Drop a disc into a column; it falls to the lowest empty cell.",
				Input: &game.ActionInput{
					Field:       columnField,
					Description: "The column, 1 to 7 from the left.",
					Values:      game.NumberedValues(boardColumns),
				},
			},
		},
		Events:  []game.Event{DroppedEvent{}},
		Reasons: []game.ResultReason{ReasonFourInARow, ReasonBoardFull},
	}
}

// New seats the first mover as red.
func (Rules) New(opts Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	seats := sides.Assign(challenger, invitees[0], game.PickFirst(opts.First, rng))
	return state{toMove: SeatRed, winningLines: [][]Cell{}}, seats, nil
}
