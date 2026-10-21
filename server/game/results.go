package game

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

// WinFor is a decisive result won by winners; with none, everyone lost.
func WinFor(reason ResultReason, winners ...Seat) Result {
	return Result{Kind: ResultDecisive, Winners: xslices.Clone(winners), Reason: reason}
}

// DrawBy is a drawn result, which has no winners.
func DrawBy(reason ResultReason) Result {
	return Result{Kind: ResultDraw, Reason: reason}
}

// LossBy is a decisive result nobody won, such as a solo game lost.
func LossBy(reason ResultReason) Result { return WinFor(reason) }

// ForfeitToRemaining is the win of every seat in all but eliminated.
func ForfeitToRemaining(all []Seat, eliminated Seat, reason ResultReason) Result {
	return WinFor(reason, slices.DeleteFunc(slices.Clone(all), func(s Seat) bool { return s == eliminated })...)
}
