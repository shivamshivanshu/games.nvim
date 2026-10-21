package chess

import "slices"

const lastRank = fileCount - 1

var promotionKinds = [...]PieceKind{Queen, Rook, Bishop, Knight}

func (p Position) LegalMoves() []Move {
	king, opponent := p.kingSquare(p.SideToMove), p.SideToMove.Opponent()
	return slices.DeleteFunc(p.pseudoLegalMoves(), func(m Move) bool {
		kingAfter := king
		if m.From == king {
			kingAfter = p.kingDestination(m)
		}
		return kingAfter != NoSquare && p.Play(m).IsAttacked(kingAfter, opponent)
	})
}

func (p Position) pseudoLegalMoves() []Move {
	var moves []Move
	for from := range Square(boardSize) {
		piece := p.Board[from]
		if piece.IsEmpty() || piece.Color != p.SideToMove {
			continue
		}
		switch piece.Kind {
		case Pawn:
			moves = p.appendPawnMoves(moves, from)
		case Knight:
			moves = p.appendLeaperMoves(moves, from, knightJumps)
		case Bishop:
			moves = p.appendSliderMoves(moves, from, bishopDirections)
		case Rook:
			moves = p.appendSliderMoves(moves, from, rookDirections)
		case Queen:
			moves = p.appendSliderMoves(moves, from, queenDirections)
		case King:
			moves = p.appendLeaperMoves(moves, from, kingSteps)
			moves = p.appendCastlingMoves(moves, from)
		}
	}
	return moves
}

func (p Position) isOpponentPiece(sq Square) bool {
	piece := p.Board[sq]
	return !piece.IsEmpty() && piece.Color != p.SideToMove
}

func (p Position) appendLeaperMoves(moves []Move, from Square, offsets []direction) []Move {
	for _, d := range offsets {
		to, ok := from.step(d)
		if ok && (p.Board[to].IsEmpty() || p.isOpponentPiece(to)) {
			moves = append(moves, Move{From: from, To: to})
		}
	}
	return moves
}

func (p Position) appendSliderMoves(moves []Move, from Square, directions []direction) []Move {
	for _, d := range directions {
		for to, ok := from.step(d); ok; to, ok = to.step(d) {
			if p.Board[to].IsEmpty() {
				moves = append(moves, Move{From: from, To: to})
				continue
			}
			if p.isOpponentPiece(to) {
				moves = append(moves, Move{From: from, To: to})
			}
			break
		}
	}
	return moves
}

func (p Position) appendPawnMoves(moves []Move, from Square) []Move {
	forward := pawnForward(p.SideToMove)
	if one, ok := from.step(direction{0, forward}); ok && p.Board[one].IsEmpty() {
		moves = appendPawnMove(moves, from, one)
		two, ok := one.step(direction{0, forward})
		if ok && from.Rank() == pawnStartRank(p.SideToMove) && p.Board[two].IsEmpty() {
			moves = append(moves, Move{From: from, To: two})
		}
	}
	for _, file := range []int{-1, 1} {
		to, ok := from.step(direction{file, forward})
		if ok && (to == p.EnPassant || p.isOpponentPiece(to)) {
			moves = appendPawnMove(moves, from, to)
		}
	}
	return moves
}

func pawnStartRank(c Color) int {
	if c == White {
		return 1
	}
	return lastRank - 1
}

func appendPawnMove(moves []Move, from, to Square) []Move {
	if to.Rank() != 0 && to.Rank() != lastRank {
		return append(moves, Move{From: from, To: to})
	}
	for _, kind := range promotionKinds {
		moves = append(moves, Move{From: from, To: to, Promotion: kind})
	}
	return moves
}

// kingDestination is where m, a king move, puts the king.
func (p Position) kingDestination(m Move) Square {
	if c, ok := p.castlingByKingMove(p.Board[m.From], m); ok {
		return c.kingTo
	}
	return m.To
}
