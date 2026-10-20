package checkers

import (
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// LegalMoves lists side's legal moves, sorted by path. Captures are mandatory,
// so when any exists only captures are returned.
func (b Board) LegalMoves(side game.Seat) []Move {
	var captures, steps []Move
	for _, sq := range b.squaresOf(side) {
		captures = append(captures, b.capturesFrom(sq)...)
		steps = append(steps, b.stepsFrom(sq)...)
	}
	moves := steps
	if len(captures) > 0 {
		moves = captures
	}
	slices.SortFunc(moves, func(x, y Move) int { return slices.Compare(x.Path, y.Path) })
	return moves
}

func (b Board) stepsFrom(from Square) []Move {
	var moves []Move
	for _, d := range b.At(from).directions() {
		if to, ok := from.step(d); ok && b.At(to) == NoPiece {
			moves = append(moves, Move{Path: []Square{from, to}})
		}
	}
	return moves
}

// capturesFrom lists every complete jump chain from origin. The origin is
// vacated so a king's chain may pass back through it. A man that reaches the
// back rank stays a man for the rest of the search, which has no forward jump
// left, so crowning ends the move.
func (b Board) capturesFrom(origin Square) []Move {
	piece := b.At(origin)
	return b.with(origin, NoPiece).extendJumps(piece, Move{Path: []Square{origin}})
}

func (b Board) extendJumps(piece Piece, sofar Move) []Move {
	var moves []Move
	for _, d := range piece.directions() {
		over, landing, ok := b.jump(piece, sofar, d)
		if !ok {
			continue
		}
		next := Move{
			Path:     append(slices.Clip(sofar.Path), landing),
			Captured: append(slices.Clip(sofar.Captured), over),
		}
		if longer := b.extendJumps(piece, next); len(longer) > 0 {
			moves = append(moves, longer...)
			continue
		}
		moves = append(moves, next)
	}
	return moves
}

func (b Board) jump(piece Piece, sofar Move, d direction) (over, landing Square, ok bool) {
	over, ok = sofar.To().step(d)
	if !ok || !b.At(over).isOpponentOf(piece) || slices.Contains(sofar.Captured, over) {
		return 0, 0, false
	}
	landing, ok = over.step(d)
	if !ok || b.At(landing) != NoPiece {
		return 0, 0, false
	}
	return over, landing, true
}
