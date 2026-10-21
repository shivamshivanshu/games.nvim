package mancala

import (
	"strconv"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

// playedOut has the mover sow its first legal pit until the game ends.
func playedOut(t *testing.T) ([]gametest.Step[Action], state) {
	t.Helper()
	return playedOutChoosing(t, func(legal []int) int { return legal[0] })
}

// playedOutChoosing has the mover sow the pit choose picks from its legal pits until the game ends.
func playedOutChoosing(t *testing.T, choose func(legal []int) int) ([]gametest.Step[Action], state) {
	t.Helper()
	s := newState(t)
	var steps []gametest.Step[Action]
	for !s.Over() {
		mover := s.toMove
		action := sow(strconv.Itoa(choose(s.View(mover).LegalPits)))
		next, _, err := s.Apply(mover, action, nil)
		if err != nil {
			t.Fatalf("step %d: %v", len(steps), err)
		}
		steps = append(steps, gametest.Step[Action]{Seat: mover, Action: action})
		s = next.(state)
	}
	return steps, s
}

func TestConformance(t *testing.T) {
	firstPits, _ := playedOut(t)
	lastPits, _ := playedOutChoosing(t, func(legal []int) int { return legal[len(legal)-1] })
	northWins := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatNorth}, Reason: ReasonMoreSeeds}
	drawn := game.Result{Kind: game.ResultDraw, Reason: ReasonEqualSeeds}
	rejected := []gametest.Step[Action]{
		{Seat: SeatNorth, Action: sow("1"), WantErr: true},
		{Seat: SeatSouth, Action: sow("7"), WantErr: true},
		{Seat: SeatSouth, Action: sow("3")},
		{Seat: SeatSouth, Action: sow("3"), WantErr: true},
		{Seat: SeatSouth, Action: sow("1")},
	}
	me := Opts{First: game.FirstMoverMe}
	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		{Name: "first legal pit every move: north wins 36 to 12", Opts: me, Challenger: alice, Invitees: []game.UserID{bob}, Seed: 1, Steps: firstPits, WantResult: &northWins},
		{Name: "last legal pit every move: 24 each, a draw", Opts: me, Challenger: alice, Invitees: []game.UserID{bob}, Seed: 2, Steps: lastPits, WantResult: &drawn},
		{Name: "rejections, an extra turn, and unfinished", Opts: me, Challenger: alice, Invitees: []game.UserID{bob}, Seed: 3, Steps: rejected},
		{Name: "random first mover", Opts: Opts{First: game.FirstMoverRandom}, Challenger: alice, Invitees: []game.UserID{bob}, Seed: 4, Steps: firstPits[:6]},
	})
}
