package hearts

import (
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

const maxGameSteps = 5000

// playToFinish plays choose for every seat, the first to act first, from
// seed, and returns the steps and every state from the first to the final one.
func playToFinish(t *testing.T, seed game.Seed) ([]gametest.Step[Action], []state) {
	t.Helper()
	steps := gametest.PlayToFinish(t, Rules{}, Opts{}, users(), seed, func(v View, _ game.Seat) Action { return choose(v) }, maxGameSteps)
	return steps, gametest.Replay(t, Rules{}.Info(), newGame(t, seed), steps, seed)
}

// stateOf is st unwrapped from the opened New hands over.
func stateOf(st game.State[Action, View]) state {
	if o, fresh := st.(opened); fresh {
		return o.state
	}
	return st.(state)
}

// privateCards are the cards of seat's hand that no other seat knows of:
// all but those passed to it, which their giver saw. A card text recurs from
// hand to hand, so the cards of the last trick, which may be the previous
// hand's, are left out too.
func privateCards(st game.State[Action, View], seat game.Seat) []string {
	s := stateOf(st)
	i := slices.Index(s.order, seat)
	var public []cards.Card
	if s.lastTrick != nil {
		for _, p := range s.lastTrick.Plays {
			public = append(public, p.Card)
		}
	}
	var out []string
	for _, c := range s.hands[i] {
		if !slices.Contains(s.received[i], c) && !slices.Contains(public, c) {
			out = append(out, c.String())
		}
	}
	return out
}

func TestConformance(t *testing.T) {
	var scripts []gametest.Script[Opts, Action]
	for _, seed := range []game.Seed{3, 8, 13} {
		steps, states := playToFinish(t, seed)
		first := states[0]
		illegal := []gametest.Step[Action]{
			{Seat: seat(1), Action: playing(first.hands[0][0].String()), WantErr: true},
			{Seat: seat(2), Action: passing(texts(first.hands[0][:3])), WantErr: true},
		}
		result, _ := states[len(states)-1].Outcome()
		scripts = append(scripts, gametest.Script[Opts, Action]{
			Name:       fmt.Sprintf("seed %d", seed),
			Challenger: users()[0],
			Invitees:   users()[1:],
			Seed:       seed,
			Steps:      append(illegal, steps...),
			WantResult: &game.Result{Kind: game.ResultDecisive, Winners: result.Winners, Reason: ReasonScoreLimit},
			PrivateOf:  gametest.Private(privateCards),
			Secrets:    []string{strconv.FormatUint(first.subseed, 10)},
		})
	}
	gametest.Run(t, Rules{}, scripts)
}

func TestTheBotGamesPlaySeveralHandsAndEveryPass(t *testing.T) {
	_, states := playToFinish(t, 3)
	final := states[len(states)-1]
	if final.handNo < 5 || slices.Max(final.scores) < ScoreLimit {
		t.Errorf("the match ended in hand %d with scores %v", final.handNo, final.scores)
	}
}

// TestViewLegalityIsWhatApplyAccepts holds the view to Apply: the seat to
// play may play exactly the cards Legal lists, a seat may pass exactly when
// MustPass asks it to, and no other seat is offered anything.
func TestViewLegalityIsWhatApplyAccepts(t *testing.T) {
	_, states := playToFinish(t, 5)
	for k, s := range states[:len(states)-1] {
		for _, at := range s.order {
			v := s.View(at)
			if !slices.Contains(s.ToAct(), at) {
				if len(v.Legal) > 0 || v.MustPass > 0 {
					t.Fatalf("step %d: %s is not to act but is offered %v, must pass %d", k, at, v.Legal, v.MustPass)
				}
				continue
			}
			for _, c := range v.MyHand {
				_, _, err := s.Apply(at, playing(c.String()), nil)
				if offered := slices.Contains(v.Legal, c); offered != (err == nil) {
					t.Fatalf("step %d %s plays %s: offered %t, Apply: %v", k, at, c, offered, err)
				}
			}
			_, _, err := s.Apply(at, passing(texts(v.MyHand[:passSize])), nil)
			if offered := v.MustPass == passSize; offered != (err == nil) {
				t.Fatalf("step %d %s passes: offered %t, Apply: %v", k, at, offered, err)
			}
		}
	}
}

// swapped trades a card of owner's hand for one of another seat's, which the
// trick number picks, both cards unknown to anyone else: neither was passed
// this hand, and neither is the two of clubs, whose holder leads. Those two
// seats alone may see the change.
func swapped(st game.State[Action, View], owner game.Seat) (game.State[Action, View], []game.Seat) {
	s := stateOf(st)
	a := slices.Index(s.order, owner)
	b := (a + 1 + s.trickNo%3) % players
	unknown := func(i int) int {
		return slices.IndexFunc(s.hands[i], func(c cards.Card) bool {
			return c != twoOfClubs && !slices.ContainsFunc(s.passes, func(p []cards.Card) bool { return slices.Contains(p, c) })
		})
	}
	x, y := unknown(a), unknown(b)
	if x < 0 || y < 0 {
		return st, nil
	}
	s.hands = slices.Clone(s.hands)
	s.hands[a], s.hands[b] = slices.Clone(s.hands[a]), slices.Clone(s.hands[b])
	s.hands[a][x], s.hands[b][y] = s.hands[b][y], s.hands[a][x]
	s.hands[a], s.hands[b] = sorted(s.hands[a]), sorted(s.hands[b])
	seen := []game.Seat{owner, s.order[b]}
	if _, fresh := st.(opened); fresh {
		return s.opened(), seen
	}
	return s, seen
}

// playsOf are the plays of every card in the hand of the view.
func playsOf(v View) []Action {
	return xslices.Map(v.MyHand, func(c cards.Card) Action { return playing(c.String()) })
}

// TestAHandReachesNoOtherSeat trades a hidden card between two seats and
// requires what each of the other two is sent to stay byte for byte the
// same. Cards passed this hand are left alone: their giver knows them.
func TestAHandReachesNoOtherSeat(t *testing.T) {
	const seed game.Seed = 21
	steps, _ := playToFinish(t, seed)
	script := gametest.Script[Opts, Action]{Challenger: users()[0], Invitees: users()[1:], Seed: seed, Steps: steps}
	gametest.NonInterference(t, Rules{}, script, gametest.Hidden[Action, View]{Perturb: swapped, Probes: playsOf})
}
