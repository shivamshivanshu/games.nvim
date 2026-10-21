package chess

type CastlingRights uint8

const (
	WhiteKingside CastlingRights = 1 << iota
	WhiteQueenside
	BlackKingside
	BlackQueenside
)

type Position struct {
	Board          [boardSize]Piece
	SideToMove     Color
	Castling       CastlingRights
	EnPassant      Square
	HalfmoveClock  int
	FullmoveNumber int
	// layout is nil for standard chess.
	layout *castlingLayout
}
