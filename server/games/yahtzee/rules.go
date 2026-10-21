// Package yahtzee is Yahtzee for one to four players: thirteen rounds of up to
// three rolls, each turn scored in one box of the seat's card.
package yahtzee

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

const (
	dieField = "die"
	boxField = "box"
)

// Rules implements game.Rules for Yahtzee.
type Rules struct{}

// Info describes Yahtzee: solo or up to four players, no options.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeYahtzee,
		Name:         "Yahtzee",
		Players:      game.PlayerRange{Min: 1, Max: 4},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Actions: []game.ActionSpec{
			{Kind: ActionRoll, Description: "Roll the dice you do not hold, up to three times a turn."},
			{
				Kind:        ActionHold,
				Description: "Hold a die between rolls, or release it.",
				Input:       &game.ActionInput{Field: dieField, Description: fmt.Sprintf("The die, 1 to %d.", DiceCount), Values: game.NumberedValues(DiceCount)},
			},
			{
				Kind:        ActionScore,
				Description: "Score the dice in an open box, ending your turn.",
				Input: &game.ActionInput{
					Field:       boxField,
					Description: "The box to fill.",
					Values:      xslices.Map(Boxes(), func(b Box) string { return string(b) }),
				},
			},
		},
		Events:  []game.Event{RolledEvent{}, ScoredEvent{}},
		Reasons: []game.ResultReason{ReasonScorecardsFull},
	}
}

// New seats the challenger as p1 and the invitees after; p1 rolls first.
func (Rules) New(_ Opts, challenger game.UserID, invitees []game.UserID, _ *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	order, seats := game.NumberSeats(game.Participants(challenger, invitees))
	return newState(order), seats, nil
}
