package sudoku

import "math/rand/v2"

const ambiguous = 2

// generate removes cells from a random solution, visiting each once in rng order and
// keeping a removal only while the puzzle stays uniquely solvable, until givens remain.
// A puzzle that becomes minimal above givens is kept as is, so the cost is bounded.
func generate(givens int, rng *rand.Rand) (puzzle, solution Grid) {
	solution = solvedGrid(rng)
	puzzle = solution
	remaining := CellCount
	for _, cell := range rng.Perm(CellCount) {
		if remaining <= givens {
			break
		}
		candidate := puzzle
		candidate[cell] = Empty
		if countSolutions(candidate, ambiguous) == 1 {
			puzzle = candidate
			remaining--
		}
	}
	return puzzle, solution
}

func (d Difficulty) givens() (int, bool) {
	switch d {
	case DifficultyEasy:
		return 40, true
	case DifficultyMedium:
		return 32, true
	case DifficultyHard:
		return 26, true
	}
	return 0, false
}
