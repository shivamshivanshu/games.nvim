package chess

import "slices"

const pliesPerMove = 2

// countedKinds are the kinds material counts, most valuable first, with their
// points; the king is not counted.
var countedKinds = []struct {
	kind   PieceKind
	points int
}{{Queen, 9}, {Rook, 5}, {Bishop, 3}, {Knight, 3}, {Pawn, 1}}

// Material is the imbalance on the board: per kind, the surplus of the side
// with more of it, shown as the opponent's pieces it is up; equal counts cancel.
func (p Position) Material() Material {
	var counts [2][King + 1]int
	for _, piece := range p.Board {
		counts[piece.Color][piece.Kind]++
	}
	material := Material{White: SideMaterial{Pieces: []PieceLetter{}}, Black: SideMaterial{Pieces: []PieceLetter{}}}
	balance := 0
	for _, counted := range countedKinds {
		surplus := counts[White][counted.kind] - counts[Black][counted.kind]
		balance += surplus * counted.points
		material.White.Pieces = append(material.White.Pieces, repeated(Piece{counted.kind, Black}, surplus)...)
		material.Black.Pieces = append(material.Black.Pieces, repeated(Piece{counted.kind, White}, -surplus)...)
	}
	material.White.Score, material.Black.Score = max(balance, 0), max(-balance, 0)
	return material
}

func repeated(p Piece, n int) []PieceLetter {
	return slices.Repeat([]PieceLetter{PieceLetter([]byte{p.letter()})}, max(n, 0))
}

// Ply is how many plies a game has behind it at p, read from its side to move
// and fullmove number.
func (p Position) Ply() int {
	ply := (p.FullmoveNumber - 1) * pliesPerMove
	if p.SideToMove == Black {
		ply++
	}
	return ply
}
