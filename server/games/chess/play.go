package chess

func (p Position) Play(m Move) Position {
	next := p
	moving := p.Board[m.From]
	isCapture := false
	if c, ok := p.castlingByKingMove(moving, m); ok {
		next.Board[c.kingFrom], next.Board[c.rookFrom] = Piece{}, Piece{}
		next.Board[c.kingTo], next.Board[c.rookTo] = moving, Piece{Kind: Rook, Color: moving.Color}
	} else {
		isCapture = !p.Board[m.To].IsEmpty()
		next.Board[m.From] = Piece{}
		next.Board[m.To] = moving
		if m.Promotion != NoPieceKind {
			next.Board[m.To] = Piece{Kind: m.Promotion, Color: moving.Color}
		}
		if moving.Kind == Pawn && m.To == p.EnPassant {
			next.Board[NewSquare(m.To.File(), m.From.Rank())] = Piece{}
			isCapture = true
		}
	}
	next.Castling &^= p.castlingRightsTiedTo(m.From) | p.castlingRightsTiedTo(m.To)
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

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
