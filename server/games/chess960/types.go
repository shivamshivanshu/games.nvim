// Package chess960 is Fischer random chess: chess from one of 960 start
// positions drawn at random, castling to the usual squares from wherever the
// king and rooks start.
package chess960

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
)

// TypeChess960 is the game type on the wire.
const TypeChess960 game.GameType = "chess960"

// StartPosition is a start position's number, 0 to 959, in Scharnagl's
// numbering; 518 is the standard array.
type StartPosition int

// View is chess's view, with the start position the game was drawn.
type View struct {
	chess.View
	StartPosition StartPosition `json:"start_position"`
	StartFEN      string        `json:"start_fen"`
}
