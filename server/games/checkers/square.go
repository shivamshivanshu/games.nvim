package checkers

// Square is a playable dark square in standard numbering, 1 to 32. Square 1 is
// b8 and 32 is g1, with Black starting on 1-12 at the top.
type Square int

// NumSquares is the count of playable dark squares.
const NumSquares = 32

const (
	boardSide     = 8
	squaresPerRow = boardSide / 2
	firstRow      = 1
	lastRow       = boardSide
)

type direction struct{ dRow, dCol int }

func towardWhite() []direction { return []direction{{1, -1}, {1, 1}} }

func towardBlack() []direction { return []direction{{-1, -1}, {-1, 1}} }

func allDirections() []direction { return append(towardBlack(), towardWhite()...) }

func squareAt(row, col int) (Square, bool) {
	onBoard := row >= firstRow && row <= lastRow && col >= 1 && col <= boardSide
	if !onBoard || (row+col)%2 == 0 {
		return 0, false
	}
	return Square((row-1)*squaresPerRow + (col-1)/2 + 1), true
}

func (s Square) row() int { return (int(s)-1)/squaresPerRow + 1 }

func (s Square) col() int {
	offset := 2 * ((int(s) - 1) % squaresPerRow)
	if s.row()%2 == 1 {
		return offset + 2
	}
	return offset + 1
}

func (s Square) step(d direction) (Square, bool) {
	return squareAt(s.row()+d.dRow, s.col()+d.dCol)
}
