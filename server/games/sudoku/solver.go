package sudoku

import (
	"iter"
	"math/bits"
	"math/rand/v2"
	"slices"
)

type digitSet uint16

const allDigits digitSet = (1<<(Size+1) - 1) &^ 1

func (s digitSet) all() iter.Seq[Digit] {
	return func(yield func(Digit) bool) {
		for rest := s; rest != 0; rest &= rest - 1 {
			if !yield(Digit(bits.TrailingZeros16(uint16(rest)))) {
				return
			}
		}
	}
}

// search is a backtracking solver's working grid; it is only ever mutated locally.
type search struct {
	grid              Grid
	rows, cols, boxes [Size]digitSet
}

// newSearch places g's givens; ok is false if any is out of range or repeats in its row, column or box.
func newSearch(g Grid) (s *search, ok bool) {
	s = &search{}
	for cell, d := range g {
		if d == Empty {
			continue
		}
		if s.candidates(cell)&(digitSet(1)<<d) == 0 {
			return nil, false
		}
		s.place(cell, d)
	}
	return s, true
}

func (s *search) place(cell int, d Digit) {
	bit := digitSet(1) << d
	s.grid[cell] = d
	s.rows[cell/Size] |= bit
	s.cols[cell%Size] |= bit
	s.boxes[boxOf(cell)] |= bit
}

func (s *search) clear(cell int) {
	bit := digitSet(1) << s.grid[cell]
	s.grid[cell] = Empty
	s.rows[cell/Size] &^= bit
	s.cols[cell%Size] &^= bit
	s.boxes[boxOf(cell)] &^= bit
}

func (s *search) candidates(cell int) digitSet {
	return allDigits &^ (s.rows[cell/Size] | s.cols[cell%Size] | s.boxes[boxOf(cell)])
}

// mostConstrained is the empty cell with the fewest candidates; ok is false when the grid is full.
func (s *search) mostConstrained() (cell int, free digitSet, ok bool) {
	best := Size + 1
	for i, d := range s.grid {
		if d != Empty {
			continue
		}
		c := s.candidates(i)
		if n := bits.OnesCount16(uint16(c)); n < best {
			cell, free, best, ok = i, c, n, true
			if n == 0 {
				break
			}
		}
	}
	return cell, free, ok
}

func (s *search) count(limit int) int {
	cell, free, ok := s.mostConstrained()
	if !ok {
		return 1
	}
	found := 0
	for d := range free.all() {
		s.place(cell, d)
		found += s.count(limit - found)
		s.clear(cell)
		if found >= limit {
			break
		}
	}
	return found
}

func (s *search) fill(rng *rand.Rand) bool {
	cell, free, ok := s.mostConstrained()
	if !ok {
		return true
	}
	for _, d := range shuffled(free, rng) {
		s.place(cell, d)
		if s.fill(rng) {
			return true
		}
		s.clear(cell)
	}
	return false
}

func shuffled(free digitSet, rng *rand.Rand) []Digit {
	digits := slices.Collect(free.all())
	rng.Shuffle(len(digits), func(i, j int) { digits[i], digits[j] = digits[j], digits[i] })
	return digits
}

// countSolutions counts the completions of g, stopping once it reaches limit; a grid with invalid givens has none.
func countSolutions(g Grid, limit int) int {
	s, ok := newSearch(g)
	if !ok {
		return 0
	}
	return s.count(limit)
}

// solvedGrid is a complete valid grid, drawn by randomized backtracking.
func solvedGrid(rng *rand.Rand) Grid {
	s, _ := newSearch(Grid{})
	if !s.fill(rng) {
		panic("sudoku: an empty grid always has a completion")
	}
	return s.grid
}
