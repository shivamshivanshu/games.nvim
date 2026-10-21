package checkers

import "github.com/shivamshivanshu/games.nvim/server/game"

// Piece is what stands on a square; the zero value is an empty square.
type Piece string

// Piece codes, as sent in View.Board.
const (
	NoPiece   Piece = ""
	BlackMan  Piece = "b"
	BlackKing Piece = "B"
	WhiteMan  Piece = "w"
	WhiteKing Piece = "W"
)

func (Piece) EnumValues() []string {
	return game.Strings(NoPiece, BlackMan, BlackKing, WhiteMan, WhiteKing)
}

func (p Piece) owner() game.Seat {
	switch p {
	case BlackMan, BlackKing:
		return SeatBlack
	case WhiteMan, WhiteKing:
		return SeatWhite
	}
	return ""
}

func (p Piece) isMan() bool { return p == BlackMan || p == WhiteMan }

func (p Piece) isOpponentOf(mover Piece) bool {
	return p != NoPiece && p.owner() != mover.owner()
}

func (p Piece) directions() []direction {
	switch p {
	case BlackMan:
		return towardWhite()
	case WhiteMan:
		return towardBlack()
	}
	return allDirections()
}

func (p Piece) crownsOn(sq Square) bool {
	switch p {
	case BlackMan:
		return sq.row() == lastRow
	case WhiteMan:
		return sq.row() == firstRow
	}
	return false
}

func (p Piece) crowned() Piece {
	switch p {
	case BlackMan:
		return BlackKing
	case WhiteMan:
		return WhiteKing
	}
	return p
}
