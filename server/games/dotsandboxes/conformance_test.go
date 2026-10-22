package dotsandboxes

import (
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

func steps(moves []move) []gametest.Step[Action] {
	out := make([]gametest.Step[Action], len(moves))
	for i, m := range moves {
		out[i] = gametest.Step[Action]{Seat: m.seat, Action: draw(m.edge)}
	}
	return out
}

func TestConformance(t *testing.T) {
	firstEdges3, _ := playedOut(t, Size3, first)
	lastEdges3, _ := playedOut(t, Size3, last)
	firstEdges5, _ := playedOut(t, Size5, first)
	firstEdges7, _ := playedOut(t, Size7, first)
	p2Wins := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{p2}, Reason: ReasonMoreBoxes}
	rejected := []gametest.Step[Action]{
		{Seat: p2, Action: draw("a1h"), WantErr: true},
		{Seat: p1, Action: draw("d1h"), WantErr: true},
		{Seat: p1, Action: draw("a1h")},
		{Seat: p2, Action: draw("a1h"), WantErr: true},
		{Seat: p2, Action: draw("a2h")},
		{Seat: p1, Action: draw("a1v")},
		{Seat: p2, Action: draw("b1v")},
		{Seat: p2, Action: draw("c3v")},
	}
	opts := func(size Size) Opts { return Opts{Size: size, First: game.FirstMoverMe} }
	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		{Name: "3 boxes, first legal edge every move: p2 wins 7 to 2", Opts: opts(Size3), Seed: 1, Steps: steps(firstEdges3), WantResult: &p2Wins},
		{Name: "3 boxes, last legal edge every move: p2 wins 6 to 3", Opts: opts(Size3), Seed: 2, Steps: steps(lastEdges3), WantResult: &p2Wins},
		{Name: "5 boxes played out", Opts: opts(Size5), Seed: 3, Steps: steps(firstEdges5), WantResult: &p2Wins},
		{Name: "7 boxes played out", Opts: opts(Size7), Seed: 4, Steps: steps(firstEdges7), WantResult: &p2Wins},
		{Name: "rejections, a box and another move, unfinished", Opts: opts(Size3), Seed: 5, Steps: rejected},
		{Name: "random first mover", Opts: Opts{Size: Size3, First: game.FirstMoverRandom}, Seed: 6, Steps: steps(firstEdges3[:5])},
	})
}
