package checkers

import "github.com/shivamshivanshu/games.nvim/server/game"

// Board holds the 32 playable squares; index i is square i+1.
type Board [NumSquares]Piece

const (
	lastBlackStart  Square = 12
	firstWhiteStart Square = 21
)

// StartBoard has Black men on 1-12 and White men on 21-32.
func StartBoard() Board {
	var b Board
	for sq := Square(1); sq <= NumSquares; sq++ {
		switch {
		case sq <= lastBlackStart:
			b = b.with(sq, BlackMan)
		case sq >= firstWhiteStart:
			b = b.with(sq, WhiteMan)
		}
	}
	return b
}

// At is the piece on sq.
func (b Board) At(sq Square) Piece { return b[sq-1] }

func (b Board) with(sq Square, p Piece) Board {
	b[sq-1] = p
	return b
}

func (b Board) squaresOf(side game.Seat) []Square {
	var out []Square
	for sq := Square(1); sq <= NumSquares; sq++ {
		if b.At(sq).owner() == side {
			out = append(out, sq)
		}
	}
	return out
}

// Play applies a legal move and reports whether it crowned the moving man.
func (b Board) Play(m Move) (Board, bool) {
	piece := b.At(m.From())
	next := b.with(m.From(), NoPiece)
	for _, sq := range m.Captured {
		next = next.with(sq, NoPiece)
	}
	crowned := piece.crownsOn(m.To())
	if crowned {
		piece = piece.crowned()
	}
	return next.with(m.To(), piece), crowned
}
