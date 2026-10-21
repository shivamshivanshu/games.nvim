package hearts

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const maxGameSteps = 5000

// playToFinish plays choose for every seat, the first to act first, with the
// rng each step gets in the server, and returns the steps and every state
// from the first to the final one.
func playToFinish(t *testing.T, seed game.Seed) ([]gametest.Step[Action], []state) {
	t.Helper()
	var st game.State[Action, View] = newGame(t, seed)
	states := []state{st.(state)}
	var steps []gametest.Step[Action]
	for seq := game.Seq(1); len(st.ToAct()) > 0; seq++ {
		if seq > maxGameSteps {
			t.Fatalf("no finish within %d steps", maxGameSteps)
		}
		by := st.ToAct()[0]
		action := choose(st.View(by))
		var err error
		if st, _, err = st.Apply(by, action, game.StepRNG(seed, seq)); err != nil {
			t.Fatalf("seq %d %s %+v: %v", seq, by, action, err)
		}
		steps = append(steps, gametest.Step[Action]{Seat: by, Action: action})
		states = append(states, st.(state))
	}
	return steps, states
}

// privateCards are the cards of seat's hand that no other seat knows of:
// all but those passed to it, which their giver saw. A card text recurs from
// hand to hand, so the cards of the last trick, which may be the previous
// hand's, are left out too.
func privateCards(st game.State[Action, View], seat game.Seat) []string {
	s, ok := st.(state)
	if !ok {
		s = st.(opened).state
	}
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

// swapped is s with a card of owner's hand traded for one of other's, both
// cards unknown to anyone else: neither was passed this hand.
func swapped(s state, owner, other int) (state, bool) {
	unknown := func(i int) int {
		return slices.IndexFunc(s.hands[i], func(c cards.Card) bool {
			return !slices.ContainsFunc(s.passes, func(p []cards.Card) bool { return slices.Contains(p, c) })
		})
	}
	a, b := unknown(owner), unknown(other)
	if a < 0 || b < 0 {
		return s, false
	}
	s.hands = slices.Clone(s.hands)
	s.hands[owner], s.hands[other] = slices.Clone(s.hands[owner]), slices.Clone(s.hands[other])
	s.hands[owner][a], s.hands[other][b] = s.hands[other][b], s.hands[owner][a]
	s.hands[owner], s.hands[other] = sorted(s.hands[owner]), sorted(s.hands[other])
	return s, true
}

func viewsOf(t *testing.T, s state, seats []int) []string {
	t.Helper()
	out := make([]string, len(seats))
	for k, i := range seats {
		out[k] = encoded(t, s.View(s.order[i]))
	}
	return out
}

// TestAHandReachesNoOtherSeat trades a hidden card between two seats and
// requires what each of the other two is sent to stay byte for byte the
// same: its view now, and the events, errors and its view after its own
// next step. Cards passed this hand are left alone: their giver knows them.
func TestAHandReachesNoOtherSeat(t *testing.T) {
	checked := 0
	steps, states := playToFinish(t, 21)
	for k, s := range states[:len(states)-1] {
		step := steps[k]
		actor := slices.Index(s.order, step.Seat)
		for owner := range players {
			other := (owner + 1 + k%3) % players
			if owner == actor || other == actor {
				continue
			}
			changed, ok := swapped(s, owner, other)
			if !ok {
				continue
			}
			watchers := slices.DeleteFunc([]int{0, 1, 2, 3}, func(i int) bool { return i == owner || i == other })
			if diff := cmp.Diff(viewsOf(t, s, watchers), viewsOf(t, changed, watchers)); diff != "" {
				t.Fatalf("step %d: views depend on the hands of %s and %s:\n%s", k, s.order[owner], s.order[other], diff)
			}
			for _, c := range s.hands[actor] {
				_, _, err := s.Apply(step.Seat, playing(c.String()), nil)
				_, _, changedErr := changed.Apply(step.Seat, playing(c.String()), nil)
				if fmt.Sprint(err) != fmt.Sprint(changedErr) {
					t.Fatalf("step %d %s plays %s: the error depends on another hand: %v / %v", k, step.Seat, c, err, changedErr)
				}
			}
			next, events, _ := s.Apply(step.Seat, step.Action, nil)
			changedNext, changedEvents, _ := changed.Apply(step.Seat, step.Action, nil)
			if diff := cmp.Diff(encodedEvents(t, events), encodedEvents(t, changedEvents)); diff != "" {
				t.Fatalf("step %d: events depend on another hand:\n%s", k, diff)
			}
			if next.(state).handNo == s.handNo {
				if diff := cmp.Diff(viewsOf(t, next.(state), watchers), viewsOf(t, changedNext.(state), watchers)); diff != "" {
					t.Fatalf("step %d: views after the step depend on another hand:\n%s", k, diff)
				}
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no hand was checked")
	}
}

type persistedStep struct {
	seat   game.Seat
	action json.RawMessage
}

func TestRestoreReplaysPersistedStepsAndPlaysOnIdentically(t *testing.T) {
	const (
		seed      game.Seed = 11
		restoreAt game.Seq  = 70
	)
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	start := func() game.ErasedState {
		st, _, err := g.New(opts, users()[0], users()[1:], game.StepRNG(seed, 0))
		if err != nil {
			t.Fatal(err)
		}
		return st
	}
	apply := func(st game.ErasedState, step persistedStep, seq game.Seq) (game.ErasedState, []json.RawMessage) {
		next, events, err := st.Apply(step.seat, step.action, game.StepRNG(seed, seq))
		if err != nil {
			t.Fatalf("seq %d: %v", seq, err)
		}
		return next, events
	}
	next := func(st game.ErasedState) persistedStep {
		by := st.ToAct()[0]
		raw, err := st.View(by)
		if err != nil {
			t.Fatal(err)
		}
		var view View
		if err := json.Unmarshal(raw, &view); err != nil {
			t.Fatal(err)
		}
		action, err := json.Marshal(choose(view))
		if err != nil {
			t.Fatal(err)
		}
		return persistedStep{seat: by, action: action}
	}

	live := start()
	var persisted []persistedStep
	for seq := game.Seq(1); seq <= restoreAt; seq++ {
		step := next(live)
		live, _ = apply(live, step, seq)
		persisted = append(persisted, step)
	}
	restored := start()
	for i, step := range persisted {
		restored, _ = apply(restored, step, game.Seq(i)+1)
	}
	for seq := restoreAt + 1; seq <= maxGameSteps; seq++ {
		for i := range players {
			liveView, _ := live.View(seat(i + 1))
			restoredView, _ := restored.View(seat(i + 1))
			if diff := cmp.Diff(string(liveView), string(restoredView)); diff != "" {
				t.Fatalf("seq %d: %s's restored view differs (-live +restored):\n%s", seq, seat(i+1), diff)
			}
		}
		if len(live.ToAct()) == 0 {
			return
		}
		step := next(live)
		var liveEvents, restoredEvents []json.RawMessage
		live, liveEvents = apply(live, step, seq)
		restored, restoredEvents = apply(restored, step, seq)
		if diff := cmp.Diff(liveEvents, restoredEvents); diff != "" {
			t.Fatalf("seq %d: restored events differ (-live +restored):\n%s", seq, diff)
		}
	}
	t.Fatalf("no finish within %d steps", maxGameSteps)
}
