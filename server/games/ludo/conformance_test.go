package ludo

import (
	"strconv"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const maxGameSteps = 5000

// firstChoice is a deterministic player: roll when due, else move the lowest movable token.
func firstChoice(v View) Action {
	if v.Phase == PhaseRoll {
		return Action{Kind: ActionRoll}
	}
	return moveToken(v.Movable[0])
}

// playToFinish plays firstChoice for every seat of users from seed.
func playToFinish(t *testing.T, users []game.UserID, seed game.Seed) []gametest.Step[Action] {
	t.Helper()
	return gametest.PlayToFinish(t, Rules{}, Opts{}, users, seed, func(v View, _ game.Seat) Action { return firstChoice(v) }, maxGameSteps)
}

func notFirst(first game.Seat) game.Seat {
	if first == SeatBlue {
		return SeatYellow
	}
	return SeatBlue
}

var playerRange = Rules{}.Info().Players

// TestOnlyTheSeatToActIsOffered plays whole games and fails unless the seat
// to act alone may roll or move at every step.
func TestOnlyTheSeatToActIsOffered(t *testing.T) {
	const seed game.Seed = 7
	for n := playerRange.Min; n <= playerRange.Max; n++ {
		users := gametest.Users(n)
		s := gametest.NewState[state](t, Rules{}, Opts{}, users, seed)
		for i, step := range playToFinish(t, users, seed) {
			for _, other := range s.order {
				view := s.View(other)
				if offered := view.CanRoll || len(view.Movable) > 0; offered != (other == step.Seat) {
					t.Fatalf("%d players, step %d: %s to act, but %s's view has can_roll %t, movable %v", n, i, step.Seat, other, view.CanRoll, view.Movable)
				}
			}
			s, _ = gametest.MustApply(t, Rules{}.Info(), s, step.Seat, step.Action, game.StepRNG(seed, game.Seq(i+1)))
		}
	}
}

func TestConformance(t *testing.T) {
	var scripts []gametest.Script[Opts, Action]
	for n := playerRange.Min; n <= playerRange.Max; n++ {
		const seed game.Seed = 7
		users := gametest.Users(n)
		steps := playToFinish(t, users, seed)
		winner := steps[len(steps)-1].Seat
		outOfTurn := gametest.Step[Action]{Seat: notFirst(steps[0].Seat), Action: Action{Kind: ActionRoll}, WantErr: true}
		scripts = append(scripts, gametest.Script[Opts, Action]{
			Name:       strconv.Itoa(n) + " players",
			Challenger: users[0],
			Invitees:   users[1:],
			Seed:       seed,
			Steps:      append([]gametest.Step[Action]{outOfTurn}, steps...),
			WantResult: &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{winner}, Reason: ReasonAllHome},
		})
	}
	gametest.Run(t, Rules{}, scripts)
}
