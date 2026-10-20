package poker

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// Rules is No-Limit Texas Hold'em as a sit-and-go for 2 to 6 seats, played
// until one seat holds every chip.
type Rules struct{}

// Info describes poker to the core: no options and no clock (D13).
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypePoker,
		Name:         "Poker",
		Players:      game.PlayerRange{Min: minPlayers, Max: maxPlayers},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Actions: []game.ActionSpec{{
			Kind:        ActionAct,
			Description: "Make your betting move, on your turn.",
			Input: &game.ActionInput{
				Field:       actionTextField,
				Description: "fold, check, call, bet N, raise N or allin, any case; N is your total for the street, e.g. raise 400.",
				Pattern:     movePattern,
			},
		}},
	}
}

// EventTypes lists one value of each event a poker state emits.
func (Rules) EventTypes() []game.Event {
	return []game.Event{
		HandStartedEvent{}, PostedEvent{}, ActedEvent{}, BoardEvent{}, ShownEvent{},
		ReturnedEvent{}, WonEvent{}, BustedEvent{}, LeftEvent{},
	}
}

// Reasons lists the result reason poker adds to the core's.
func (Rules) Reasons() []game.ResultReason { return []game.ResultReason{ReasonAllChips} }

// New seats the challenger as p1 and the invitees clockwise after, draws the
// button and the sub-seed every hand's deck comes from, and deals hand 1.
func (Rules) New(_ Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	users := append([]game.UserID{challenger}, invitees...)
	if len(users) < minPlayers || len(users) > maxPlayers {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("poker needs %d to %d players, got %d", minPlayers, maxPlayers, len(users))}
	}
	s := state{n: len(users), subseed: rng.Uint64()}
	seats := make(map[game.UserID]game.Seat, len(users))
	for i, user := range users {
		s.users[i] = user
		s.hand.chips.stack[i] = startingStack
		seats[user] = seatName(i)
	}
	s, _ = s.dealHand("", 1, rng.IntN(s.n))
	s, _ = s.advance("", ReasonAllChips)
	return s, seats, nil
}
