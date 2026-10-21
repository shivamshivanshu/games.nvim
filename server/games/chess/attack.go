package chess

import "slices"

type direction struct{ file, rank int }

var (
	rookDirections   = []direction{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	bishopDirections = []direction{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
	queenDirections  = slices.Concat(rookDirections, bishopDirections)
	kingSteps        = queenDirections
	knightJumps      = []direction{{1, 2}, {2, 1}, {2, -1}, {1, -2}, {-1, -2}, {-2, -1}, {-2, 1}, {-1, 2}}
)

func (s Square) step(d direction) (Square, bool) {
	file, rank := s.File()+d.file, s.Rank()+d.rank
	if file < 0 || file >= fileCount || rank < 0 || rank >= fileCount {
		return NoSquare, false
	}
	return NewSquare(file, rank), true
}

func pawnForward(c Color) int {
	if c == White {
		return 1
	}
	return -1
}

func (p Position) IsAttacked(sq Square, by Color) bool {
	return p.attackedByPawn(sq, by) ||
		p.attackedByLeaper(sq, Piece{Kind: Knight, Color: by}, knightJumps) ||
		p.attackedByLeaper(sq, Piece{Kind: King, Color: by}, kingSteps) ||
		p.attackedBySlider(sq, by, Rook, rookDirections) ||
		p.attackedBySlider(sq, by, Bishop, bishopDirections)
}

func (p Position) InCheck() bool { return p.kingAttacked(p.SideToMove) }

// CheckSquare is the square of the side to move's king while it is in check, else nil.
func (p Position) CheckSquare() *string {
	king := p.kingSquare(p.SideToMove)
	if king == NoSquare || !p.IsAttacked(king, p.SideToMove.Opponent()) {
		return nil
	}
	return new(king.String())
}

func (p Position) kingAttacked(c Color) bool {
	king := p.kingSquare(c)
	return king != NoSquare && p.IsAttacked(king, c.Opponent())
}

func (p Position) kingSquare(c Color) Square {
	king := Piece{Kind: King, Color: c}
	for sq := range Square(boardSize) {
		if p.Board[sq] == king {
			return sq
		}
	}
	return NoSquare
}

func (p Position) attackedByPawn(sq Square, by Color) bool {
	pawn := Piece{Kind: Pawn, Color: by}
	for _, file := range []int{-1, 1} {
		from, ok := sq.step(direction{file, -pawnForward(by)})
		if ok && p.Board[from] == pawn {
			return true
		}
	}
	return false
}

func (p Position) attackedByLeaper(sq Square, attacker Piece, offsets []direction) bool {
	for _, d := range offsets {
		from, ok := sq.step(d)
		if ok && p.Board[from] == attacker {
			return true
		}
	}
	return false
}

func (p Position) attackedBySlider(sq Square, by Color, kind PieceKind, directions []direction) bool {
	for _, d := range directions {
		blocker := p.firstPieceAlong(sq, d)
		if blocker.Color == by && (blocker.Kind == kind || blocker.Kind == Queen) {
			return true
		}
	}
	return false
}

func (p Position) firstPieceAlong(sq Square, d direction) Piece {
	for next, ok := sq.step(d); ok; next, ok = next.step(d) {
		if !p.Board[next].IsEmpty() {
			return p.Board[next]
		}
	}
	return Piece{}
}
