package battleship

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const cellField = "cell"

// Rules is Battleship on 10x10 with a fleet of 5, 4, 3, 3 and 2: each seat
// places its fleet unseen, then the seats take turns firing one shot each
// until one fleet is sunk.
type Rules struct{}

// Info implements game.Rules.
func (Rules) Info() game.Info {
	cellInput := func(description string) *game.ActionInput {
		c := coords()
		return &game.ActionInput{
			Field:       cellField,
			Description: fmt.Sprintf("%s, %s: the column's letter from a at the left, then the row's number from 1 at the top.", description, c.Range()),
			Pattern:     c.Pattern(),
		}
	}
	return game.Info{
		Type:         TypeBattleship,
		Name:         "Battleship",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Options:      []game.OptionSpec{game.FirstMoverOption()},
		Actions: []game.ActionSpec{
			{Kind: ActionPlaceAcross, Description: "Place your next ship lying across, to the right of the cell.", Input: cellInput("Its left end")},
			{Kind: ActionPlaceDown, Description: "Place your next ship lying down, below the cell.", Input: cellInput("Its top end")},
			{Kind: ActionRandom, Description: "Place the rest of your fleet at random."},
			{Kind: ActionFire, Description: "Fire at a cell of the opponent's waters you have not fired at.", Input: cellInput("The cell")},
		},
		Events:  []game.Event{ReadyEvent{}, ShotEvent{}},
		Reasons: []game.ResultReason{ReasonFleetSunk},
	}
}

// New implements game.Rules. Seat p1 fires first: the challenger, or the
// opponent, as Opts.First picks.
func (Rules) New(opts Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	return newState(), sides.Assign(opts.First, challenger, invitees, rng), nil
}
