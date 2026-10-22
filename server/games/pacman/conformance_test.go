package pacman

import (
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

const unlimited = 100_000

func TestConformance(t *testing.T) {
	lost := game.LossBy(ReasonGameOver)
	won := game.WinFor(ReasonCleared, game.SeatSolo)
	lostSteps, lostEnd := autoplay(t, 0, unlimited)
	wonSteps, wonEnd := autoplay(t, 3, unlimited)
	lostResult, _ := lostEnd.Outcome()
	wonResult, _ := wonEnd.Outcome()
	if lostResult.Reason != ReasonGameOver || wonResult.Reason != ReasonCleared {
		t.Fatalf("seed 0 ends %+v and seed 3 %+v; want a loss and a win", lostResult, wonResult)
	}
	tick := gametest.Step[Action]{Kind: gametest.StepTick}
	rejected := func(a Action) gametest.Step[Action] {
		return gametest.Step[Action]{Seat: game.SeatSolo, Action: a, WantErr: true}
	}
	withPauses := []gametest.Step[Action]{
		rejected(Action{Kind: ActionPause}),
		rejected(Action{Kind: ActionTurn, Direction: "north"}),
		rejected(Action{Kind: "jump"}),
		{Seat: game.SeatSolo, Action: turn(grid.Left)},
		tick,
		{Seat: game.SeatSolo, Action: Action{Kind: ActionPause}},
		{Seat: game.SeatSolo, Action: turn(grid.Right)},
		tick,
		{Kind: gametest.StepPause},
		rejected(Action{Kind: ActionPause}),
		{Seat: game.SeatSolo, Action: turn(grid.Up)},
		tick, tick, tick,
	}
	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		{Name: "caught on every life", Challenger: player, Seed: 0, Steps: lostSteps, WantResult: &lost},
		{Name: "every level cleared", Challenger: player, Seed: 3, Steps: wonSteps, WantResult: &won},
		{Name: "pauses and rejected actions, unfinished", Challenger: player, Seed: 5, Steps: withPauses},
	})
}
