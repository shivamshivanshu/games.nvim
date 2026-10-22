package grid

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRows(t *testing.T) {
	got := Rows(2, 3, func(p Point) int { return 10*p.Row + p.Col })
	if diff := cmp.Diff([][]int{{0, 1, 2}, {10, 11, 12}}, got); diff != "" {
		t.Errorf("Rows (-want +got):\n%s", diff)
	}
}

func TestRowsOfCopiesFlat(t *testing.T) {
	flat := []int{1, 2, 3, 4, 5, 6}
	got := RowsOf(flat, 2)
	if diff := cmp.Diff([][]int{{1, 2}, {3, 4}, {5, 6}}, got); diff != "" {
		t.Errorf("RowsOf (-want +got):\n%s", diff)
	}
	got[0] = append(got[0], 0)
	got[1][0] = 0
	if !cmp.Equal(flat, []int{1, 2, 3, 4, 5, 6}) {
		t.Errorf("RowsOf shares flat: %v", flat)
	}
}
