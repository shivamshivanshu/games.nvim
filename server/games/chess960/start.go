package chess960

import "github.com/shivamshivanshu/games.nvim/server/games/chess"

const (
	positionCount  = 960
	bishopSquares  = 4
	queenSquares   = 6
	filesPerSquare = 2
)

// knightPairs are the two knights' places among the five files left once the
// bishops and the queen stand, as Scharnagl's numbering orders them.
func knightPairs() [10][2]int {
	return [10][2]int{{0, 1}, {0, 2}, {0, 3}, {0, 4}, {1, 2}, {1, 3}, {1, 4}, {2, 3}, {2, 4}, {3, 4}}
}

// backRank is white's back rank in start position n: a bishop on a light and
// one on a dark square, the queen, the knights, then a rook, the king and a
// rook on the three files left, so the king stands between the rooks.
func backRank(n StartPosition) chess.BackRank {
	var rank chess.BackRank
	rest := int(n)
	rest, light := rest/bishopSquares, rest%bishopSquares
	rank[light*filesPerSquare+1] = chess.Bishop
	rest, dark := rest/bishopSquares, rest%bishopSquares
	rank[dark*filesPerSquare] = chess.Bishop
	knights, queen := rest/queenSquares, rest%queenSquares
	rank[emptyFiles(rank)[queen]] = chess.Queen
	empty := emptyFiles(rank)
	for _, i := range knightPairs()[knights] {
		rank[empty[i]] = chess.Knight
	}
	rookKingRook := [...]chess.PieceKind{chess.Rook, chess.King, chess.Rook}
	for i, file := range emptyFiles(rank) {
		rank[file] = rookKingRook[i]
	}
	return rank
}

func emptyFiles(rank chess.BackRank) []int {
	var files []int
	for file, kind := range rank {
		if kind == chess.NoPieceKind {
			files = append(files, file)
		}
	}
	return files
}
