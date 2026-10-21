// Package liarsdice is Liar's Dice as Perudo for two to six players: bid on
// the dice under every cup, ones wild, and call dudo on a bid you doubt.
package liarsdice

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const bidField = "bid"

// Rules implements game.Rules for Liar's Dice.
type Rules struct{}

// Info describes Liar's Dice: two to six players, no options.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeLiarsDice,
		Name:         "Liar's Dice",
		Players:      game.PlayerRange{Min: 2, Max: 6},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Actions: []game.ActionSpec{
			{
				Kind:        ActionBid,
				Description: "Raise the bid, on your turn.",
				Input: &game.ActionInput{
					Field:       bidField,
					Description: "Quantity x face, e.g. 3x5 for three fives; ones are wild.",
					Pattern:     bidPattern,
				},
			},
			{Kind: ActionDudo, Description: "Challenge the standing bid: every cup is revealed."},
		},
		Events:  []game.Event{BidEvent{}, ChallengedEvent{}, OutEvent{}},
		Reasons: []game.ResultReason{ReasonLastWithDice},
	}
}

// New seats the challenger as p1 and the invitees after, and rolls every cup; p1 opens.
func (Rules) New(_ Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	order, seats := game.NumberSeats(game.Participants(challenger, invitees))
	return newState(order, rng), seats, nil
}
