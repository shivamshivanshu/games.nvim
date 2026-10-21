package g2048

import (
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// playedOut moves the first movable direction at every step from seed's start, as the core
// would number the steps, until the board is stuck or limit moves are made.
func playedOut(t *testing.T, seed game.Seed, limit int) ([]grid.Direction, state) {
	t.Helper()
	s := newState(t, seed)
	var moves []grid.Direction
	for !s.Over() && len(moves) < limit {
		d := s.board.movable()[0]
		s, _ = gametest.MustApply(t, Rules{}.Info(), s, game.SeatSolo, move(string(d)), game.StepRNG(seed, game.Seq(len(moves)+1)))
		moves = append(moves, d)
	}
	return moves, s
}

func moveSteps(moves []grid.Direction) []gametest.Step[Action] {
	steps := make([]gametest.Step[Action], len(moves))
	for i, d := range moves {
		steps[i] = gametest.Step[Action]{Seat: game.SeatSolo, Action: move(string(d))}
	}
	return steps
}

func rejected(action Action) gametest.Step[Action] {
	return gametest.Step[Action]{Seat: game.SeatSolo, Action: action, WantErr: true}
}

func TestConformance(t *testing.T) {
	lost := game.LossBy(ReasonNoMoves)
	const unlimited = 10000
	stuckMoves, _ := playedOut(t, 1, unlimited)
	withRejections := func(seed game.Seed) []gametest.Step[Action] {
		moves, _ := playedOut(t, seed, unlimited)
		start := newState(t, seed).board
		steps := []gametest.Step[Action]{rejected(move("north")), rejected(Action{Kind: "slide", Direction: "up"})}
		for _, d := range grid.Directions() {
			if next, _ := start.slid(d); next == start {
				steps = append(steps, rejected(move(string(d))))
			}
		}
		return append(steps, moveSteps(moves)...)
	}
	someMoves, _ := playedOut(t, 3, 5)
	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		{Name: "played until no move is left", Challenger: solver, Seed: 1, Steps: moveSteps(stuckMoves), WantResult: &lost},
		{Name: "rejected moves cost nothing", Challenger: solver, Seed: 2, Steps: withRejections(2), WantResult: &lost},
		{Name: "unfinished", Challenger: solver, Seed: 3, Steps: moveSteps(someMoves)},
	})
}
