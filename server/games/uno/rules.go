// Package uno is Uno for two to six players under the official rules, played
// as one hand: the first seat to play its last card wins.
package uno

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// Rules implements game.Rules for Uno.
type Rules struct{}

// EventTypes implements game.EventCatalog.
func (Rules) EventTypes() []game.Event {
	return []game.Event{
		StartedEvent{}, PlayedEvent{}, DrewEvent{}, PassedEvent{}, SkippedEvent{},
		ReversedEvent{}, ColorEvent{}, ChallengedEvent{}, ReshuffledEvent{}, LeftEvent{},
	}
}

// Reasons implements game.ReasonCatalog.
func (Rules) Reasons() []game.ResultReason { return []game.ResultReason{ReasonWentOut} }

// Info describes Uno: two to six players, no options and no clock.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeUno,
		Name:         "Uno",
		Players:      game.PlayerRange{Min: minPlayers, Max: maxPlayers},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Actions: []game.ActionSpec{
			{
				Kind:        ActionPlay,
				Description: "Play a card from your hand that matches the colour or the face on the pile, or a wild, on your turn.",
				Input: &game.ActionInput{
					Field:       cardField,
					Description: "A colour letter r, y, g or b and a face 0-9, s (skip), r (reverse) or +2, e.g. r5 or g+2; or w or w+4 then the colour it names, e.g. w b. Any case.",
					Pattern:     playSyntax.String(),
				},
			},
			{Kind: ActionDraw, Description: "Draw a card instead of playing; you may play it at once if it matches."},
			{Kind: ActionPass, Description: "Keep the card you just drew and end your turn."},
			{Kind: ActionChallenge, Description: "Challenge the Wild Draw Four played on you: if its player held a card of the colour before it, they draw four; if not, you draw six."},
			{Kind: ActionAccept, Description: "Draw the four cards of the Wild Draw Four played on you, and lose your turn."},
			{
				Kind:        ActionColor,
				Description: "Name the colour in play for a Wild turned up to start, as its first player.",
				Input:       &game.ActionInput{Field: colorField, Description: "R, Y, G or B.", Values: colorNames()},
			},
		},
	}
}

// New seats the challenger as p1 and the invitees clockwise after, draws the
// dealer and the sub-seed the deck is shuffled from, and deals.
func (Rules) New(_ Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	users := append([]game.UserID{challenger}, invitees...)
	if len(users) < minPlayers || len(users) > maxPlayers {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("uno takes %d to %d players, got %d", minPlayers, maxPlayers, len(users))}
	}
	order, seats := game.NumberSeats(users)
	subseed := rng.Uint64()
	s, events := deal(order, rng.IntN(len(order)), subseed)
	return opened{state: s, deal: events}, seats, nil
}
