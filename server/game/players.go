package game

import (
	"fmt"
	"strconv"

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

// NumberedSeat is the seat at index i of a seating order numbered p1, p2, ….
func NumberedSeat(i int) Seat { return Seat("p" + strconv.Itoa(i+1)) }

// NumberSeats seats users as p1, p2, … in their given order, and returns that order.
func NumberSeats(users []UserID) ([]Seat, map[UserID]Seat) {
	order := make([]Seat, len(users))
	seats := make(map[UserID]Seat, len(users))
	for i, user := range users {
		order[i] = NumberedSeat(i)
		seats[user] = order[i]
	}
	return order, seats
}

// SeatSolo is the one seat of a solo game.
const SeatSolo Seat = "solver"

// SoloSeat seats the challenger of a solo game.
func SoloSeat(challenger UserID) map[UserID]Seat { return map[UserID]Seat{challenger: SeatSolo} }
