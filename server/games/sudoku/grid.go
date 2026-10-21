package sudoku

const (
	// Size is the side of the grid and the number of digits.
	Size = 9
	// BoxSize is the side of one box.
	BoxSize = 3
	// CellCount is the number of cells in the grid.
	CellCount = Size * Size
)

// Digit is a cell's value: 1 to Size, or Empty.
type Digit int

// Empty is an unfilled cell.
const Empty Digit = 0

// Grid holds every cell in row-major order.
type Grid [CellCount]Digit

func cellIndex(row, col int) int { return (row-1)*Size + col - 1 }

// conflicts marks every filled cell whose digit repeats in its row, column or box.
func conflicts(g Grid) [CellCount]bool {
	var marked [CellCount]bool
	for a := range CellCount {
		for b := a + 1; b < CellCount; b++ {
			if g[a] != Empty && g[a] == g[b] && sharesUnit(a, b) {
				marked[a], marked[b] = true, true
			}
		}
	}
	return marked
}

func sharesUnit(a, b int) bool {
	return a/Size == b/Size || a%Size == b%Size || boxOf(a) == boxOf(b)
}

func boxOf(cell int) int {
	row, col := cell/Size, cell%Size
	return row/BoxSize*BoxSize + col/BoxSize
}
