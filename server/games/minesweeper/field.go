package minesweeper

import (
	"math/rand/v2"
	"slices"
)

// placeMines scatters the level's mines from rng over every cell but first and its
// neighbours, so the first reveal always opens a 0.
func placeMines(lvl level, first int, rng *rand.Rand) []bool {
	spared := append(lvl.neighbours(first), first)
	var candidates []int
	for cell := range lvl.cells() {
		if !slices.Contains(spared, cell) {
			candidates = append(candidates, cell)
		}
	}
	mines := make([]bool, lvl.cells())
	for _, k := range rng.Perm(len(candidates))[:lvl.mines] {
		mines[candidates[k]] = true
	}
	return mines
}

// uncover opens cell, and every cell a 0 floods into without crossing a flag, and returns
// how many it opened. A mine is not opened: it explodes.
func (s *state) uncover(cell int) int {
	if s.mines[cell] {
		if s.exploded == noCell {
			s.exploded = cell
		}
		return 0
	}
	opened := 0
	pending := []int{cell}
	for len(pending) > 0 {
		c := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if s.open[c] || s.flags[c] {
			continue
		}
		s.open[c] = true
		opened++
		if s.minesAround(c) == 0 {
			pending = append(pending, s.lvl.neighbours(c)...)
		}
	}
	return opened
}

func (s state) isMine(cell int) bool { return s.mines != nil && s.mines[cell] }

func (s state) minesAround(cell int) int { return s.countAround(cell, s.isMine) }

func (s state) flagsAround(cell int) int {
	return s.countAround(cell, func(n int) bool { return s.flags[n] })
}

func (s state) countAround(cell int, holds func(int) bool) int {
	count := 0
	for _, n := range s.lvl.neighbours(cell) {
		if holds(n) {
			count++
		}
	}
	return count
}

// coveredAround lists cell's neighbours that are neither open nor flagged.
func (s state) coveredAround(cell int) []int {
	var out []int
	for _, n := range s.lvl.neighbours(cell) {
		if !s.open[n] && !s.flags[n] {
			out = append(out, n)
		}
	}
	return out
}
