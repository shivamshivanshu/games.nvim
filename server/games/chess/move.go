package chess

type Move struct {
	From, To  Square
	Promotion PieceKind
}

func (m Move) UCI() string {
	uci := m.From.String() + m.To.String()
	if m.Promotion == NoPieceKind {
		return uci
	}
	return uci + string(Piece{Kind: m.Promotion, Color: Black}.letter())
}
