// Package hearts is Hearts for exactly four players: pass three cards left,
// right, across, then hold; avoid hearts and the queen of spades, or take
// them all to shoot the moon; play to 100, and the lowest score wins.
package hearts

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// Rules implements game.Rules for Hearts.
type Rules struct{}

// EventTypes implements game.EventCatalog.
func (Rules) EventTypes() []game.Event {
	return []game.Event{
		HandStartedEvent{}, PassChosenEvent{}, PassedEvent{}, PlayedEvent{},
		TrickWonEvent{}, HandScoredEvent{}, LeftEvent{},
	}
}

// Reasons implements game.ReasonCatalog.
func (Rules) Reasons() []game.ResultReason { return []game.ResultReason{ReasonScoreLimit} }

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
	}
}

// New seats the challenger as p1 and the invitees clockwise after, draws the
// sub-seed every hand's deck is shuffled from, and deals hand 1.
func (Rules) New(_ Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	users := append([]game.UserID{challenger}, invitees...)
	if len(users) != players {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("hearts takes exactly %d players, got %d", players, len(users))}
	}
	order, seats := game.NumberSeats(users)
	return newState(order, rng.Uint64()), seats, nil
}
