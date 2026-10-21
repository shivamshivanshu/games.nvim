package reversi

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
	"github.com/shivamshivanshu/games.nvim/server/games/discs"
)

// startBoard has white on d4 and e5, black on e4 and d5.
func startBoard() discs.Board {
	return discs.NewBoard(boardSize).
		With(discs.SeatWhite, grid.Point{Row: 3, Col: 3}, grid.Point{Row: 4, Col: 4}).
		With(discs.SeatBlack, grid.Point{Row: 3, Col: 4}, grid.Point{Row: 4, Col: 3})
}

func directions() []grid.Point {
	return []grid.Point{{Row: -1, Col: -1}, {Row: -1}, {Row: -1, Col: 1}, {Col: -1}, {Col: 1}, {Row: 1, Col: -1}, {Row: 1}, {Row: 1, Col: 1}}
}

// flips are the opponent's discs a disc of seat on the empty point at would
// turn over: every unbroken run of them, in each direction, that one of seat's
// discs closes.
func flips(b discs.Board, seat game.Seat, at grid.Point) []grid.Point {
	turned := []grid.Point{}
	for _, d := range directions() {
		run := []grid.Point{}
		p := step(at, d)
		for b.Contains(p) && b.At(p) == discs.Opponent(seat) {
			run = append(run, p)
			p = step(p, d)
		}
		if b.Contains(p) && b.At(p) == seat {
			turned = append(turned, run...)
		}
	}
	return turned
}

func step(p, d grid.Point) grid.Point { return grid.Point{Row: p.Row + d.Row, Col: p.Col + d.Col} }

// legalPoints are the empty points where seat's disc would flip at least one, row by row.
func legalPoints(b discs.Board, seat game.Seat) []grid.Point {
	legal := []grid.Point{}
	for _, p := range b.Empty() {
		if len(flips(b, seat, p)) > 0 {
			legal = append(legal, p)
		}
	}
	return legal
}
