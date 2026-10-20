package ludo

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// TypeLudo is the game type on the wire.
const TypeLudo game.GameType = "ludo"

// Rules is the Ludo game: 2 to 4 seats racing four tokens home, first finisher wins.
type Rules struct{}

// Info describes Ludo to the core.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeLudo,
		Name:         "Ludo",
		Players:      game.PlayerRange{Min: minPlayers, Max: maxPlayers},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort, game.CapClock},
		Options:      []game.OptionSpec{game.TimeControlOption()},
	}
}

// New seats the challenger and invitees in seat-colour order and draws the first seat to roll.
func (Rules) New(_ Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	users := append([]game.UserID{challenger}, invitees...)
	if len(users) < minPlayers || len(users) > maxPlayers {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("ludo needs %d to %d players, got %d", minPlayers, maxPlayers, len(users))}
	}
	order := seatOrder()[:len(users)]
	seats := make(map[game.UserID]game.Seat, len(users))
	for i, user := range users {
		seats[user] = order[i]
	}
	return newState(order, order[rng.IntN(len(order))]), seats, nil
}
