package chess

import "fmt"

type Square int8

const (
	NoSquare  Square = -1
	boardSize        = 64
	fileCount        = 8
)

func NewSquare(file, rank int) Square { return Square(rank*fileCount + file) }

func (s Square) File() int { return int(s) % fileCount }

func (s Square) Rank() int { return int(s) / fileCount }

func (s Square) String() string {
	if s < 0 || s >= boardSize {
		return "-"
	}
	return string([]byte{byte('a' + s.File()), byte('1' + s.Rank())})
}

func ParseSquare(s string) (Square, error) {
	if len(s) != 2 || s[0] < 'a' || s[0] > 'h' || s[1] < '1' || s[1] > '8' {
		return NoSquare, fmt.Errorf("invalid square %q", s)
	}
	return NewSquare(int(s[0]-'a'), int(s[1]-'1')), nil
}
