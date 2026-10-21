package grid

// Adjacency is which cells count as a cell's neighbours.
type Adjacency int

const (
	// Orthogonal neighbours share an edge: up to 4.
	Orthogonal Adjacency = iota
	// King neighbours share an edge or a corner: up to 8.
	King
)

// Neighbours lists the cells adjacent to p on a board of rows by cols, in row-major order.
func Neighbours(p Point, rows, cols int, adjacency Adjacency) []Point {
	var out []Point
	for dRow := -1; dRow <= 1; dRow++ {
		for dCol := -1; dCol <= 1; dCol++ {
			n := Point{Row: p.Row + dRow, Col: p.Col + dCol}
			diagonal := dRow != 0 && dCol != 0
			if n == p || diagonal && adjacency == Orthogonal || !n.within(rows, cols) {
				continue
			}
			out = append(out, n)
		}
	}
	return out
}

func (p Point) within(rows, cols int) bool {
	return p.Row >= 0 && p.Row < rows && p.Col >= 0 && p.Col < cols
}
