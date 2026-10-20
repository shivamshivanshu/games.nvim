package chess

import "strings"

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
	kind := strings.IndexByte(pieceLetters, l)
	if kind < int(Pawn) {
		return Piece{}, false
	}
	return Piece{Kind: PieceKind(kind), Color: color}, true
}
