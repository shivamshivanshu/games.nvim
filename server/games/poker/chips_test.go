package poker

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const (
	randomGamesPerSize = 100
	// leaveOneIn is how rarely a random step is a seat leaving instead of the seat to act moving.
	leaveOneIn = 30
)

func chipsInPlay(s state) Chips {
	var total Chips
	for i := range s.n {
		total += s.hand.chips.stack[i] + s.hand.chips.committed[i]
	}
	return total
}

// randomMove is one of v's legal moves, with a bet or raise anywhere in its range.
func randomMove(rng *rand.Rand, v View) string {
	kind := v.Legal[rng.IntN(len(v.Legal))]
	if !raisesTo(kind) {
		return string(kind)
	}
	return fmt.Sprintf("%s %d", kind, v.MinRaiseTo+Chips(rng.Int64N(int64(v.MaxRaiseTo-v.MinRaiseTo+1))))
}

// randomLeaver is a seat still in the game, which need not be the one to act.
func randomLeaver(rng *rand.Rand, s state) int {
	alive := s.aliveFrom(0)
	return alive[rng.IntN(len(alive))]
}

// TestChipsInPlayAndForfeitedStacksAlwaysTotalTheStartingChips plays random
// 3 to 6 player games of random legal moves, in which seats resign or time
// out at any point of a hand. After every step the chips in play, stacks and
// pots, plus the stacks that left with their seats, total 1000 per player.
func TestChipsInPlayAndForfeitedStacksAlwaysTotalTheStartingChips(t *testing.T) {
	reasons := []game.ResultReason{game.ReasonResign, game.ReasonTimeout}
	for n := 3; n <= maxPlayers; n++ {
		for seed := range uint64(randomGamesPerSize) {
			rng := rand.New(rand.NewPCG(seed, uint64(n)))
			players := gametest.Users(n)
			st, _, err := Rules{}.New(Opts{}, players[0], players[1:], rng)
			if err != nil {
				t.Fatal(err)
			}
			s, forfeited := st.(state), Chips(0)
			for step := 0; !s.Over(); step++ {
				if step > maxGameSteps {
					t.Fatalf("%d players seed %d: no finish within %d steps", n, seed, maxGameSteps)
				}
				var what string
				if rng.IntN(leaveOneIn) == 0 {
					i := randomLeaver(rng, s)
					forfeited += s.hand.chips.stack[i]
					next, _ := s.Eliminate(seatName(i), reasons[rng.IntN(len(reasons))])
					s, what = next.(state), string(seatName(i))+" leaves"
				} else {
					seat := s.ToAct()[0]
					text := randomMove(rng, s.View(seat))
					next, _, err := s.Apply(seat, Action{Kind: ActionAct, Text: text}, nil)
					if err != nil {
						t.Fatalf("%d players seed %d step %d: legal %s %q rejected: %v", n, seed, step, seat, text, err)
					}
					s, what = next.(state), fmt.Sprintf("%s %s", seat, text)
				}
				if got := chipsInPlay(s) + forfeited; got != startingStack*Chips(n) {
					t.Fatalf("%d players seed %d step %d (%s): %d in play + %d forfeited = %d, want %d",
						n, seed, step, what, chipsInPlay(s), forfeited, got, startingStack*Chips(n))
				}
			}
		}
	}
}
