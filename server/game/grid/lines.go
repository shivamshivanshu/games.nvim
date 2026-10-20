// Package grid holds board helpers shared by grid games, indexed cells[row][col] from 0.
package grid

type Point struct{ Row, Col int }

type direction struct{ dRow, dCol int }

var lineDirections = []direction{{0, 1}, {1, 0}, {1, 1}, {1, -1}}

// WinningLines returns every maximal run of at least k cells equal to the one at last
// that passes through last: horizontal, vertical, diagonal, then anti-diagonal,
// each ordered from its top (or left) end. A last cell holding empty never wins.
func WinningLines[T comparable](cells [][]T, last Point, k int, empty T) [][]Point {
	if !inBounds(cells, last) {
		return nil
	}
	owner := cells[last.Row][last.Col]
	if owner == empty {
		return nil
	}
	var lines [][]Point
	for _, d := range lineDirections {
		if line := runThrough(cells, last, d, owner); len(line) >= k {
			lines = append(lines, line)
		}
	}
	return lines
}

func runThrough[T comparable](cells [][]T, from Point, d direction, owner T) []Point {
	start := from
	for p := start.step(d, -1); owns(cells, p, owner); p = p.step(d, -1) {
		start = p
	}
	var line []Point
	for p := start; owns(cells, p, owner); p = p.step(d, 1) {
		line = append(line, p)
	}
	return line
}

func (p Point) step(d direction, sign int) Point {
	return Point{Row: p.Row + sign*d.dRow, Col: p.Col + sign*d.dCol}
}

func owns[T comparable](cells [][]T, p Point, owner T) bool {
	return inBounds(cells, p) && cells[p.Row][p.Col] == owner
}

func inBounds[T any](cells [][]T, p Point) bool {
	return p.Row >= 0 && p.Row < len(cells) && p.Col >= 0 && p.Col < len(cells[p.Row])
}
