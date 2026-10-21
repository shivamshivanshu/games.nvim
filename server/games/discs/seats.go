// Package discs holds what gomoku and reversi share: a square board two players
// fill with black and white discs, each placed by naming its cell.
package discs

import "github.com/shivamshivanshu/games.nvim/server/game"

// SeatBlack moves first; SeatWhite second. NoDisc is an empty cell.
const (
	SeatBlack game.Seat = "black"
	SeatWhite game.Seat = "white"
	NoDisc    game.Seat = ""
)

// Sides are black, which moves first, and white.
var Sides = game.Pair{SeatBlack, SeatWhite}
