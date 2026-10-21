// Package yahtzee is Yahtzee for one to four players: thirteen rounds of up to
// three rolls, each turn scored in one box of the seat's card.
package yahtzee

import (
	"fmt"
	"math/rand/v2"
	"strconv"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

const (
	minPlayers = 1
	maxPlayers = 4
	dieField   = "die"
	boxField   = "box"
)

// Rules implements game.Rules for Yahtzee.
type Rules struct{}

// EventTypes implements game.EventCatalog.
func (Rules) EventTypes() []game.Event { return []game.Event{RolledEvent{}, ScoredEvent{}} }

// Reasons implements game.ReasonCatalog.
func (Rules) Reasons() []game.ResultReason { return []game.ResultReason{ReasonScorecardsFull} }

func dieNumbers() []string {
	numbers := make([]string, DiceCount)
	for i := range numbers {
		numbers[i] = strconv.Itoa(i + 1)
	}
	return numbers
}

// Info describes Yahtzee: solo or up to four players, no options.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeYahtzee,
		Name:         "Yahtzee",
		Players:      game.PlayerRange{Min: minPlayers, Max: maxPlayers},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Actions: []game.ActionSpec{
			{Kind: ActionRoll, Description: "Roll the dice you do not hold, up to three times a turn."},
			{
				Kind:        ActionHold,
				Description: "Hold a die between rolls, or release it.",
				Input:       &game.ActionInput{Field: dieField, Description: fmt.Sprintf("The die, 1 to %d.", DiceCount), Values: dieNumbers()},
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
	}
}

// New seats the challenger as p1 and the invitees after; p1 rolls first.
func (Rules) New(_ Opts, challenger game.UserID, invitees []game.UserID, _ *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	users := append([]game.UserID{challenger}, invitees...)
	if len(users) < minPlayers || len(users) > maxPlayers {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("yahtzee takes %d to %d players, got %d", minPlayers, maxPlayers, len(users))}
	}
	order, seats := game.NumberSeats(users)
	return newState(order), seats, nil
}
