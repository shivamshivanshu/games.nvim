package sudoku

import (
	"strings"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const (
	// wikipediaPuzzle and its solution are the Wikipedia "Sudoku" article's example.
	wikipediaPuzzle   = "53..7....6..195....98....6.8...6...34..8.3..17...2...6.6....28....419..5....8..79"
	wikipediaSolution = "534678912672195348198342567859761423426853791713924856961537284287419635345286179"
	// seventeenClues is a well-known minimal puzzle; no 16-clue puzzle has a unique solution.
	seventeenClues = "..............3.85..1.2.......5.7.....4...1...9.......5......73..2.1........4...9"
	// minimalAboveHard is a seed whose hard puzzle turns minimal before reaching its target.
	minimalAboveHard game.Seed = 113
)

func gridOf(t testing.TB, s string) Grid {
	t.Helper()
	if len(s) != CellCount {
		t.Fatalf("grid %q has %d cells", s, len(s))
	}
	var g Grid
	for i := range CellCount {
		if s[i] != '.' {
			g[i] = Digit(s[i] - '0')
		}
	}
	return g
}

func TestCountSolutions(t *testing.T) {
	oneClueShort := []byte(seventeenClues)
	oneClueShort[14] = '.'
	duplicateInFullGrid := []byte(wikipediaSolution)
	duplicateInFullGrid[1] = '5'
	outOfRange := []byte(wikipediaPuzzle)
	outOfRange[2] = '0' + 10
	tests := []struct {
		name  string
		grid  string
		limit int
		want  int
	}{
		{"unique puzzle", wikipediaPuzzle, ambiguous, 1},
		{"its solution", wikipediaSolution, ambiguous, 1},
		{"minimal puzzle", seventeenClues, ambiguous, 1},
		{"minimal puzzle less a clue stops at the limit", string(oneClueShort), ambiguous, ambiguous},
		{"full grid with a repeated digit", string(duplicateInFullGrid), ambiguous, 0},
		{"repeated given in a box", "5" + strings.Repeat(".", Size) + "5" + strings.Repeat(".", CellCount-Size-2), ambiguous, 0},
		{"given out of range", string(outOfRange), ambiguous, 0},
		{"empty grid stops at the limit", strings.Repeat(".", CellCount), 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countSolutions(gridOf(t, tt.grid), tt.limit); got != tt.want {
				t.Errorf("countSolutions = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGeneratedPuzzlesHaveAUniqueSolutionAndReachTheirTarget(t *testing.T) {
	for _, d := range []Difficulty{DifficultyEasy, DifficultyMedium, DifficultyHard} {
		target, _ := d.givens()
		for _, seed := range []game.Seed{1, 2, 3, 4, 5, minimalAboveHard} {
			puzzle, solution := generate(target, game.StepRNG(seed, 0))
			checkComplete(t, solution)
			givens := 0
			for cell, digit := range puzzle {
				if digit == Empty {
					continue
				}
				givens++
				if digit != solution[cell] {
					t.Fatalf("%s seed %d: given %d at cell %d, solution has %d", d, seed, digit, cell, solution[cell])
				}
			}
			if n := countSolutions(puzzle, ambiguous); n != 1 {
				t.Errorf("%s seed %d: %d solutions, want 1", d, seed, n)
			}
			if givens < target || (givens > target && !isMinimal(puzzle)) {
				t.Errorf("%s seed %d: %d givens, want %d or a minimal puzzle", d, seed, givens, target)
			}
		}
	}
}

func TestHardPuzzleThatTurnsMinimalIsKeptAboveTarget(t *testing.T) {
	target, _ := DifficultyHard.givens()
	puzzle, _ := generate(target, game.StepRNG(minimalAboveHard, 0))
	if givens := countGivens(puzzle); givens <= target || !isMinimal(puzzle) {
		t.Errorf("seed %d: %d givens, minimal=%t; want a minimal puzzle above %d", minimalAboveHard, givens, isMinimal(puzzle), target)
	}
}

func TestGenerationIsDeterministicFromTheRNG(t *testing.T) {
	first, firstSolution := generate(32, game.StepRNG(1, 0))
	again, againSolution := generate(32, game.StepRNG(1, 0))
	other, _ := generate(32, game.StepRNG(2, 0))
	if first != again || firstSolution != againSolution {
		t.Error("the same seed generated different puzzles")
	}
	if first == other {
		t.Error("seeds 1 and 2 generated the same puzzle")
	}
}

func checkComplete(t *testing.T, g Grid) {
	t.Helper()
	var rows, cols, boxes [Size]digitSet
	for cell, d := range g {
		bit := digitSet(1) << d
		rows[cell/Size] |= bit
		cols[cell%Size] |= bit
		boxes[boxOf(cell)] |= bit
	}
	for i := range Size {
		if rows[i] != allDigits || cols[i] != allDigits || boxes[i] != allDigits {
			t.Fatalf("solution is not a valid complete grid: %v", g)
		}
	}
}

func isMinimal(puzzle Grid) bool {
	for cell, d := range puzzle {
		if d == Empty {
			continue
		}
		fewer := puzzle
		fewer[cell] = Empty
		if countSolutions(fewer, ambiguous) == 1 {
			return false
		}
	}
	return true
}

func countGivens(g Grid) int {
	n := 0
	for _, d := range g {
		if d != Empty {
			n++
		}
	}
	return n
}
