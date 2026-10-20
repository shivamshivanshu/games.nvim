package chess

import "slices"

const lastRank = fileCount - 1

var promotionKinds = [...]PieceKind{Queen, Rook, Bishop, Knight}

type castling struct {
	letter           byte
	color            Color
	right            CastlingRights
	kingFrom, kingTo Square
	rookFrom, rookTo Square
	mustBeEmpty      []Square
	kingPath         []Square
}

var castlings = [...]castling{
	castlingFor('K', White, WhiteKingside, "e1", "g1", "h1", "f1", []string{"f1", "g1"}),
	castlingFor('Q', White, WhiteQueenside, "e1", "c1", "a1", "d1", []string{"d1", "c1", "b1"}),
	castlingFor('k', Black, BlackKingside, "e8", "g8", "h8", "f8", []string{"f8", "g8"}),
	castlingFor('q', Black, BlackQueenside, "e8", "c8", "a8", "d8", []string{"d8", "c8", "b8"}),
}

func castlingFor(letter byte, color Color, right CastlingRights, kingFrom, kingTo, rookFrom, rookTo string, betweenFromKing []string) castling {
	mustBeEmpty := make([]Square, len(betweenFromKing))
	for i, s := range betweenFromKing {
		mustBeEmpty[i] = mustParseSquare(s)
	}
	return castling{
		letter:      letter,
		color:       color,
		right:       right,
		kingFrom:    mustParseSquare(kingFrom),
		kingTo:      mustParseSquare(kingTo),
		rookFrom:    mustParseSquare(rookFrom),
		rookTo:      mustParseSquare(rookTo),
		mustBeEmpty: mustBeEmpty,
		kingPath:    mustBeEmpty[:2],
	}
}

func mustParseSquare(s string) Square {
	sq, err := ParseSquare(s)
	if err != nil {
		panic(err)
	}
	return sq
}

func (p Position) LegalMoves() []Move {
	king, opponent := p.kingSquare(p.SideToMove), p.SideToMove.Opponent()
	return slices.DeleteFunc(p.pseudoLegalMoves(), func(m Move) bool {
		kingAfter := king
		if m.From == king {
			kingAfter = m.To
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

func (p Position) appendCastlingMoves(moves []Move, from Square) []Move {
	for _, c := range castlings {
		if c.color == p.SideToMove && c.kingFrom == from && p.canCastle(c) {
			moves = append(moves, Move{From: c.kingFrom, To: c.kingTo})
		}
	}
	return moves
}

func (p Position) canCastle(c castling) bool {
	opponent := c.color.Opponent()
	return p.Castling&c.right != 0 &&
		p.Board[c.rookFrom] == Piece{Kind: Rook, Color: c.color} &&
		!slices.ContainsFunc(c.mustBeEmpty, func(sq Square) bool { return !p.Board[sq].IsEmpty() }) &&
		!p.IsAttacked(c.kingFrom, opponent) &&
		!slices.ContainsFunc(c.kingPath, func(sq Square) bool { return p.IsAttacked(sq, opponent) })
}
