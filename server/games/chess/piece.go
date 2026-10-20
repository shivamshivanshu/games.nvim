package chess

type Color uint8

const (
	White Color = iota
	Black
)

func (c Color) Opponent() Color { return c ^ 1 }

type PieceKind uint8

const (
	NoPieceKind PieceKind = iota
	Pawn
	Knight
	Bishop
	Rook
	Queen
	King
)

type Piece struct {
	Kind  PieceKind
	Color Color
}

const pieceLetters = "?PNBRQK"

func (p Piece) IsEmpty() bool { return p.Kind == NoPieceKind }

func (p Piece) letter() byte {
	l := pieceLetters[p.Kind]
	if p.Color == Black {
		l += 'a' - 'A'
	}
	return l
}

func pieceFromLetter(l byte) (Piece, bool) {
	color := White
	if l >= 'a' && l <= 'z' {
		color = Black
		l -= 'a' - 'A'
	}
	for kind := Pawn; kind <= King; kind++ {
		if pieceLetters[kind] == l {
			return Piece{Kind: kind, Color: color}, true
		}
	}
	return Piece{}, false
}
