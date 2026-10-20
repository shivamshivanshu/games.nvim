package game

import (
	"fmt"

	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

// PlayersOf seats users in their given order; seats must hold exactly those users.
func PlayersOf(users []UserID, seats map[UserID]Seat) ([]Player, error) {
	if len(seats) != len(users) {
		return nil, fmt.Errorf("game assigned %d seats for %d players", len(seats), len(users))
	}
	return xslices.MapErr(users, func(user UserID) (Player, error) {
		seat, ok := seats[user]
		if !ok {
			return Player{}, fmt.Errorf("game assigned no seat to %s", user)
		}
		return Player{User: user, Seat: seat}, nil
	})
}

// Users lists the players' users in seating order.
func Users(players []Player) []UserID {
	return xslices.Map(players, func(p Player) UserID { return p.User })
}
