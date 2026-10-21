package grid

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNeighbours(t *testing.T) {
	tests := []struct {
		name      string
		p         Point
		rows      int
		cols      int
		adjacency Adjacency
		want      []Point
	}{
		{"orthogonal in the middle", Point{1, 1}, 3, 3, Orthogonal, []Point{{0, 1}, {1, 0}, {1, 2}, {2, 1}}},
		{"king in the middle", Point{1, 1}, 3, 3, King, []Point{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 2}, {2, 0}, {2, 1}, {2, 2}}},
		{"orthogonal in a corner", Point{0, 0}, 3, 3, Orthogonal, []Point{{0, 1}, {1, 0}}},
		{"king in a corner", Point{2, 3}, 3, 4, King, []Point{{1, 2}, {1, 3}, {2, 2}}},
		{"king on an edge of a wide board", Point{0, 5}, 2, 9, King, []Point{{0, 4}, {0, 6}, {1, 4}, {1, 5}, {1, 6}}},
		{"a lone cell has none", Point{0, 0}, 1, 1, King, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, Neighbours(tt.p, tt.rows, tt.cols, tt.adjacency)); diff != "" {
				t.Errorf("Neighbours (-want +got):\n%s", diff)
			}
		})
	}
}
