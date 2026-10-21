package chess

import (
	"fmt"
	"slices"
)

// variant is the rule set a game castles by.
type variant uint8

const (
	standard variant = iota
	chess960
)

const (
	standardKingFile          = 4
	standardKingsideRookFile  = 7
	standardQueensideRookFile = 0
)

// castleSide is where a castle on one side lands the king and the rook,
// whichever files they start on.
type castleSide struct {
	san                    string
	kingToFile, rookToFile int
}

type castling struct {
	letter           byte
	color            Color
	right            CastlingRights
	san              string
	kingFrom, kingTo Square
	rookFrom, rookTo Square
	// move is the castle as the king's move: onto kingTo in standard chess,
	// onto rookFrom in Chess960, where kingTo may be a plain king move too.
	move        Move
	mustBeEmpty []Square
	kingPath    []Square
}

// castlingLayout is every castle's start and landing squares, fixed for a
// game by its start position.
type castlingLayout struct {
	variant   variant
	castlings [4]castling
}

var standardLayout = newCastlingLayout(standard, standardKingFile, standardKingsideRookFile, standardQueensideRookFile)

func newCastlingLayout(v variant, kingFile, kingsideRookFile, queensideRookFile int) *castlingLayout {
	kingside := castleSide{san: kingsideCastleSAN, kingToFile: 6, rookToFile: 5}
	queenside := castleSide{san: queensideCastleSAN, kingToFile: 2, rookToFile: 3}
	return &castlingLayout{variant: v, castlings: [...]castling{
		newCastling(v, 'K', White, WhiteKingside, kingFile, kingsideRookFile, kingside),
		newCastling(v, 'Q', White, WhiteQueenside, kingFile, queensideRookFile, queenside),
		newCastling(v, 'k', Black, BlackKingside, kingFile, kingsideRookFile, kingside),
		newCastling(v, 'q', Black, BlackQueenside, kingFile, queensideRookFile, queenside),
	}}
}

// newCastling needs empty every square the king or the rook crosses or lands
// on, but for the two of them, and the king safe on each it crosses or lands on.
func newCastling(v variant, letter byte, color Color, right CastlingRights, kingFile, rookFile int, side castleSide) castling {
	rank := homeRank(color)
	c := castling{
		letter:   letter,
		color:    color,
		right:    right,
		san:      side.san,
		kingFrom: NewSquare(kingFile, rank),
		kingTo:   NewSquare(side.kingToFile, rank),
		rookFrom: NewSquare(rookFile, rank),
		rookTo:   NewSquare(side.rookToFile, rank),
	}
	c.move = Move{From: c.kingFrom, To: c.kingTo}
	if v == chess960 {
		c.move.To = c.rookFrom
	}
	isMover := func(sq Square) bool { return sq == c.kingFrom || sq == c.rookFrom }
	c.mustBeEmpty = slices.DeleteFunc(slices.Concat(rankSpan(c.kingFrom, c.kingTo), rankSpan(c.rookFrom, c.rookTo)), isMover)
	c.kingPath = slices.DeleteFunc(rankSpan(c.kingFrom, c.kingTo), func(sq Square) bool { return sq == c.kingFrom })
	return c
}

// rankSpan is every square from a to b on their rank, both included.
func rankSpan(a, b Square) []Square {
	lo, hi := min(a, b), max(a, b)
	span := make([]Square, 0, hi-lo+1)
	for sq := lo; sq <= hi; sq++ {
		span = append(span, sq)
	}
	return span
}

func homeRank(c Color) int {
	if c == White {
		return 0
	}
	return lastRank
}

func (p Position) castlings() []castling {
	if p.layout == nil {
		return standardLayout.castlings[:]
	}
	return p.layout.castlings[:]
}

func (p Position) isChess960() bool { return p.layout != nil && p.layout.variant == chess960 }

// BackRank is a side's first rank, from the a-file to the h-file.
type BackRank [fileCount]PieceKind

// Chess960Position is the start position with white's pieces on rank, pawns
// in front of them and black mirroring white. It castles Chess960's way with
// the rooks either side of the king, a castle written as the king's move onto
// its own rook.
func Chess960Position(rank BackRank) (Position, error) {
	kingFile := slices.Index(rank[:], King)
	var rookFiles []int
	for file, kind := range rank {
		if kind == Rook {
			rookFiles = append(rookFiles, file)
		}
	}
	if kingFile < 0 || slices.Index(rank[kingFile+1:], King) >= 0 || len(rookFiles) != 2 ||
		kingFile < rookFiles[0] || kingFile > rookFiles[1] {
		return Position{}, fmt.Errorf("back rank %v: want one king between two rooks", rank)
	}
	p := Position{
		SideToMove:     White,
		Castling:       WhiteKingside | WhiteQueenside | BlackKingside | BlackQueenside,
		EnPassant:      NoSquare,
		FullmoveNumber: 1,
		layout:         newCastlingLayout(chess960, kingFile, rookFiles[1], rookFiles[0]),
	}
	for file, kind := range rank {
		p.Board[NewSquare(file, homeRank(White))] = Piece{Kind: kind, Color: White}
		p.Board[NewSquare(file, pawnStartRank(White))] = Piece{Kind: Pawn, Color: White}
		p.Board[NewSquare(file, pawnStartRank(Black))] = Piece{Kind: Pawn, Color: Black}
		p.Board[NewSquare(file, homeRank(Black))] = Piece{Kind: kind, Color: Black}
	}
	return p, nil
}

func (p Position) appendCastlingMoves(moves []Move, from Square) []Move {
	for _, c := range p.castlings() {
		if c.color == p.SideToMove && c.kingFrom == from && p.canCastle(c) {
			moves = append(moves, c.move)
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

// castlingByKingMove finds the castle m is. A castle written onto the rook's
// square needs the rook there: onto that square left empty, m is a plain step.
func (p Position) castlingByKingMove(moving Piece, m Move) (castling, bool) {
	if moving.Kind != King {
		return castling{}, false
	}
	for _, c := range p.castlings() {
		if c.move.From == m.From && c.move.To == m.To &&
			(c.move.To != c.rookFrom || p.Board[m.To] == Piece{Kind: Rook, Color: moving.Color}) {
			return c, true
		}
	}
	return castling{}, false
}

func (p Position) castlingRightsTiedTo(sq Square) CastlingRights {
	var rights CastlingRights
	for _, c := range p.castlings() {
		if sq == c.kingFrom || sq == c.rookFrom {
			rights |= c.right
		}
	}
	return rights
}
