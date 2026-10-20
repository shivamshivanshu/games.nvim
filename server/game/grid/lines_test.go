package grid

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

const emptyCell = -1

// board maps '.' to emptyCell and each digit to that player, so player 0 is a real owner.
func board(rows ...string) [][]int {
	cells := make([][]int, len(rows))
	for i, r := range rows {
		cells[i] = make([]int, len(r))
		for j, ch := range r {
			cells[i][j] = emptyCell
			if ch != '.' {
				cells[i][j] = int(ch - '0')
			}
		}
	}
	return cells
}

func row(r int, cols ...int) []Point {
	line := make([]Point, len(cols))
	for i, c := range cols {
		line[i] = Point{r, c}
	}
	return line
}

func TestWinningLines(t *testing.T) {
	tests := []struct {
		name  string
		cells [][]int
		last  Point
		k     int
		want  [][]Point
	}{
		{
			name:  "no line shorter than k",
			cells: board("00.", "...", "..."),
			last:  Point{0, 1},
			k:     3,
		},
		{
			name:  "player 0 wins a row of exactly k",
			cells: board("000", "11.", "..."),
			last:  Point{0, 2},
			k:     3,
			want:  [][]Point{row(0, 0, 1, 2)},
		},
		{
			name:  "run longer than k is returned whole",
			cells: board(".00000.", "......."),
			last:  Point{0, 3},
			k:     4,
			want:  [][]Point{row(0, 1, 2, 3, 4, 5)},
		},
		{
			name:  "another owner's run through last is ignored",
			cells: board("111", "0..", "0.."),
			last:  Point{2, 0},
			k:     3,
		},
		{
			name:  "double line in row and column",
			cells: board("0..", "0..", "000"),
			last:  Point{2, 0},
			k:     3,
			want:  [][]Point{row(2, 0, 1, 2), {{0, 0}, {1, 0}, {2, 0}}},
		},
		{
			name:  "both diagonals through the centre",
			cells: board("0.0", ".0.", "0.0"),
			last:  Point{1, 1},
			k:     3,
			want:  [][]Point{{{0, 0}, {1, 1}, {2, 2}}, {{0, 2}, {1, 1}, {2, 0}}},
		},
		{
			name:  "empty last cell never wins",
			cells: board("...", "...", "..."),
			last:  Point{1, 1},
			k:     1,
		},
		{
			name:  "last outside the board",
			cells: board("000"),
			last:  Point{1, 0},
			k:     3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, WinningLines(tt.cells, tt.last, tt.k, emptyCell)); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}
