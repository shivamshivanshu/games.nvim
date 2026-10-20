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

func boxOf(cell int) int {
	row, col := cell/Size, cell%Size
	return row/BoxSize*BoxSize + col/BoxSize
}
