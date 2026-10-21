// Package discs holds what gomoku and reversi share: a square board two players
// fill with black and white discs, each placed by naming its cell.
package discs

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// SeatBlack moves first; SeatWhite second. NoDisc is an empty cell.
const (
	SeatBlack game.Seat = "black"
	SeatWhite game.Seat = "white"
	NoDisc    game.Seat = ""
)

// Seats lists both seats, black first.
func Seats() []game.Seat { return []game.Seat{SeatBlack, SeatWhite} }

// Opponent is the other seat.
func Opponent(seat game.Seat) game.Seat {
	if seat == SeatBlack {
		return SeatWhite
	}
	return SeatBlack
}

// SeatPlayers gives black, which moves first, to the player first picks and
// white to the other; gameType names the game when there is not exactly one opponent.
func SeatPlayers(gameType game.GameType, first game.FirstMover, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (map[game.UserID]game.Seat, error) {
	if len(invitees) != 1 {
		return nil, game.BadRequestError{Msg: fmt.Sprintf("%s needs exactly one opponent, got %d", gameType, len(invitees))}
	}
	black, white := challenger, invitees[0]
	if !game.PickFirst(first, rng) {
		black, white = white, black
	}
	return map[game.UserID]game.Seat{black: SeatBlack, white: SeatWhite}, nil
}
