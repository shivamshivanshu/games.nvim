package minesweeper

import "github.com/shivamshivanshu/games.nvim/server/game/grid"

type level struct{ rows, cols, mines int }

func (d Difficulty) level() (level, bool) {
	switch d {
	case DifficultyEasy:
		return level{rows: 9, cols: 9, mines: 10}, true
	case DifficultyMedium:
		return level{rows: 16, cols: 16, mines: 40}, true
	case DifficultyHard:
		return level{rows: 16, cols: 30, mines: 99}, true
	}
	return level{}, false
}

func (l level) cells() int { return l.rows * l.cols }

func (l level) index(p grid.Point) int { return p.Row*l.cols + p.Col }

func (l level) point(i int) grid.Point { return grid.Point{Row: i / l.cols, Col: i % l.cols} }

func (l level) neighbours(i int) []int {
	points := grid.Neighbours(l.point(i), l.rows, l.cols, grid.King)
	out := make([]int, len(points))
	for k, p := range points {
		out[k] = l.index(p)
	}
	return out
}
