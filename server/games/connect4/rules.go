package connect4

import (
	"fmt"
	"math/rand/v2"
	"strconv"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// TypeConnect4 is the game type of connect4.
const TypeConnect4 game.GameType = "connect4"

// Rules is the connect4 game: 7 columns by 6 rows, four in a row wins.
type Rules struct{}

// EventTypes lists the events connect4 emits.
func (Rules) EventTypes() []game.Event { return []game.Event{DroppedEvent{}} }

// Reasons lists the result reasons connect4 adds to the core's.
func (Rules) Reasons() []game.ResultReason {
	return []game.ResultReason{ReasonFourInARow, ReasonBoardFull}
}

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
					Values:      columnNames(),
				},
			},
		},
	}
}

func columnNames() []string {
	names := make([]string, boardColumns)
	for i := range names {
		names[i] = strconv.Itoa(i + 1)
	}
	return names
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
