// Package hearts is Hearts for exactly four players: pass three cards left,
// right, across, then hold; avoid hearts and the queen of spades, or take
// them all to shoot the moon; play to 100, and the lowest score wins.
package hearts

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// Rules implements game.Rules for Hearts.
type Rules struct{}

// Info describes Hearts: exactly four players, no options and no clock.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeHearts,
		Name:         "Hearts",
		Players:      game.PlayerRange{Min: players, Max: players},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Actions: []game.ActionSpec{
			{
				Kind:        ActionPass,
				Description: "Choose the three cards you pass at the start of the hand; every seat chooses at once.",
				Input: &game.ActionInput{
					Field:       cardsField,
					Description: "Three cards you hold, apart, e.g. 2c qs ah: a rank 2-9, t, j, q, k or a, then a suit c, d, h or s, any case.",
					Pattern:     passSyntax.String(),
				},
			},
			{
				Kind:        ActionPlay,
				Description: "Play a card to the trick, on your turn.",
				Input: &game.ActionInput{
					Field:       cardField,
					Description: "A card you hold, e.g. qs: a rank 2-9, t, j, q, k or a, then a suit c, d, h or s, any case.",
					Pattern:     playSyntax.String(),
				},
			},
		},
		Events: []game.Event{
			HandStartedEvent{}, PassChosenEvent{}, PassedEvent{}, PlayedEvent{},
			TrickWonEvent{}, HandScoredEvent{}, LeftEvent{},
		},
		Reasons: []game.ResultReason{ReasonScoreLimit},
	}
}

// New seats the challenger as p1 and the invitees clockwise after, draws the
// sub-seed every hand's deck is shuffled from, and deals hand 1.
func (Rules) New(_ Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	order, seats := game.NumberSeats(game.Participants(challenger, invitees))
	return newState(order, rng.Uint64()).opened(), seats, nil
}
