package grid

import "slices"

// Rows is rows of cols values from the top left, each the one at gives for its point.
func Rows[T any](rows, cols int, at func(Point) T) [][]T {
	out := make([][]T, rows)
	for r := range out {
		out[r] = make([]T, cols)
		for c := range out[r] {
			out[r][c] = at(Point{Row: r, Col: c})
		}
	}
	return out
}

// RowsOf is flat, row by row from the top left, cut into fresh rows of cols values.
func RowsOf[T any](flat []T, cols int) [][]T {
	out := make([][]T, len(flat)/cols)
	for r := range out {
		out[r] = slices.Clone(flat[r*cols : (r+1)*cols])
	}
	return out
}
