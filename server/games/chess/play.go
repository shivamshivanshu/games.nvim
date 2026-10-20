package chess

func (p Position) Play(m Move) Position {
	next := p
	moving := p.Board[m.From]
	isCapture := !p.Board[m.To].IsEmpty()
	next.Board[m.From] = Piece{}
	next.Board[m.To] = moving
	if m.Promotion != NoPieceKind {
		next.Board[m.To] = Piece{Kind: m.Promotion, Color: moving.Color}
	}
	if moving.Kind == Pawn && m.To == p.EnPassant {
		next.Board[NewSquare(m.To.File(), m.From.Rank())] = Piece{}
		isCapture = true
	}
	if c, ok := castlingByKingMove(moving, m); ok {
		next.Board[c.rookTo] = next.Board[c.rookFrom]
		next.Board[c.rookFrom] = Piece{}
	}
	next.Castling &^= castlingRightsTiedTo(m.From) | castlingRightsTiedTo(m.To)
	next.EnPassant = NoSquare
	if moving.Kind == Pawn && abs(m.To.Rank()-m.From.Rank()) == 2 {
		next.EnPassant = NewSquare(m.From.File(), (m.From.Rank()+m.To.Rank())/2)
	}
	next.HalfmoveClock++
	if moving.Kind == Pawn || isCapture {
		next.HalfmoveClock = 0
	}
	if p.SideToMove == Black {
		next.FullmoveNumber++
	}
	next.SideToMove = p.SideToMove.Opponent()
	return next
}

func castlingByKingMove(moving Piece, m Move) (castling, bool) {
	if moving.Kind != King {
		return castling{}, false
	}
	for _, c := range castlings {
		if c.kingFrom == m.From && c.kingTo == m.To {
			return c, true
		}
	}
	return castling{}, false
}

func castlingRightsTiedTo(sq Square) CastlingRights {
	var rights CastlingRights
	for _, c := range castlings {
		if sq == c.kingFrom || sq == c.rookFrom {
			rights |= c.right
		}
	}
	return rights
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
