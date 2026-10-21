package mancala

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const (
	pitsPerSide = 6
	startSeeds  = 4
	holes       = 2 * (pitsPerSide + 1)
)

// board holds every hole counter-clockwise from south's pit 1: south's pits, south's store,
// north's pits, then north's store.
type board [holes]int

func startBoard() board {
	var b board
	for _, seat := range []game.Seat{SeatSouth, SeatNorth} {
		for pit := 1; pit <= pitsPerSide; pit++ {
			b[pitHole(seat, pit)] = startSeeds
		}
	}
	return b
}

func opponent(seat game.Seat) game.Seat {
	if seat == SeatSouth {
		return SeatNorth
	}
	return SeatSouth
}

// firstHole is the hole of seat's pit 1.
func firstHole(seat game.Seat) int {
	if seat == SeatSouth {
		return 0
	}
	return pitsPerSide + 1
}

// pitHole is the hole of seat's 1-based pit.
func pitHole(seat game.Seat, pit int) int { return firstHole(seat) + pit - 1 }

func storeHole(seat game.Seat) int { return firstHole(seat) + pitsPerSide }

// ownPit reports whether hole is one of seat's pits, not a store.
func ownPit(seat game.Seat, hole int) bool {
	first := firstHole(seat)
	return hole >= first && hole < first+pitsPerSide
}

// oppositeHole is the pit facing hole across the board: south's pit 1 faces north's pit 6.
func oppositeHole(hole int) int { return 2*pitsPerSide - hole }

func (b board) pits(seat game.Seat) []int {
	first := firstHole(seat)
	return slices.Clone(b[first : first+pitsPerSide])
}

func (b board) side(seat game.Seat) Side {
	return Side{Pits: b.pits(seat), Store: b[storeHole(seat)]}
}

func (b board) pitsEmpty(seat game.Seat) bool {
	return !slices.ContainsFunc(b.pits(seat), func(seeds int) bool { return seeds > 0 })
}

// legalPits are seat's 1-based pits that hold seeds.
func (b board) legalPits(seat game.Seat) []int {
	legal := []int{}
	for i, seeds := range b.pits(seat) {
		if seeds > 0 {
			legal = append(legal, i+1)
		}
	}
	return legal
}

// sown empties seat's pit and drops its seeds one per hole counter-clockwise, skipping the
// opponent's store but not the emptied pit; it returns the board and the hole the last seed fell in.
func (b board) sown(seat game.Seat, pit int) (board, int) {
	hole := pitHole(seat, pit)
	seeds := b[hole]
	b[hole] = 0
	skipped := storeHole(opponent(seat))
	for seeds > 0 {
		hole = (hole + 1) % holes
		if hole == skipped {
			continue
		}
		b[hole]++
		seeds--
	}
	return b, hole
}

// captured moves the last seed and the seeds facing it into seat's store when the last seed fell
// in one of seat's empty pits and the facing pit holds seeds; it returns the board and the seeds taken.
func (b board) captured(seat game.Seat, last int) (board, int) {
	facing := oppositeHole(last)
	if !ownPit(seat, last) || b[last] != 1 || b[facing] == 0 {
		return b, 0
	}
	taken := b[last] + b[facing]
	b[last], b[facing] = 0, 0
	b[storeHole(seat)] += taken
	return b, taken
}

// collected moves the seeds in seat's pits into its store; it returns the board and their number.
func (b board) collected(seat game.Seat) (board, int) {
	seeds := 0
	for pit := 1; pit <= pitsPerSide; pit++ {
		hole := pitHole(seat, pit)
		seeds += b[hole]
		b[hole] = 0
	}
	b[storeHole(seat)] += seeds
	return b, seeds
}
