package uno

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const maxGameSteps = 5000

// playToFinish plays choose for every seat with the rng each step gets in the
// server, and returns the steps and every state from the first to the final one.
func playToFinish(t *testing.T, n int, seed game.Seed) ([]gametest.Step[Action], []state) {
	t.Helper()
	var st game.State[Action, View] = newGame(t, n, seed)
	states := []state{st.(state)}
	var steps []gametest.Step[Action]
	for seq := game.Seq(1); len(st.ToAct()) > 0; seq++ {
		if seq > maxGameSteps {
			t.Fatalf("%d players: no finish within %d steps", n, maxGameSteps)
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

func TestConformance(t *testing.T) {
	var scripts []gametest.Script[Opts, Action]
	for n := minPlayers; n <= maxPlayers; n++ {
		for _, seed := range []game.Seed{3, 8} {
			steps, states := playToFinish(t, n, seed)
			first := states[0]
			idle := first.order[(first.turn+1)%n]
			illegal := []gametest.Step[Action]{
				{Seat: idle, Action: kind(ActionDraw), WantErr: true},
				{Seat: first.order[first.turn], Action: kind(ActionPass), WantErr: true},
				{Seat: first.order[first.turn], Action: playing("x9"), WantErr: true},
			}
			result, _ := states[len(states)-1].Outcome()
			scripts = append(scripts, gametest.Script[Opts, Action]{
				Name:       fmt.Sprintf("%d players seed %d", n, seed),
				Challenger: users(n)[0],
				Invitees:   users(n)[1:],
				Seed:       seed,
				Steps:      append(illegal, steps...),
				WantResult: &game.Result{Kind: game.ResultDecisive, Winners: result.Winners, Reason: ReasonWentOut},
				Secrets:    []string{strconv.FormatUint(first.subseed, 10)},
			})
		}
	}
	gametest.Run(t, Rules{}, scripts)
}

// TestTheBotGamesReachTheTurnsThatHideOrRevealCards keeps the bot games,
// which the conformance and secrecy tests play, exercising those turns.
func TestTheBotGamesReachTheTurnsThatHideOrRevealCards(t *testing.T) {
	seen := map[string]bool{}
	for n := minPlayers; n <= maxPlayers; n++ {
		for _, seed := range []game.Seed{3, 8} {
			_, states := playToFinish(t, n, seed)
			for k := 1; k < len(states); k++ {
				prev, next := states[k-1], states[k]
				seen["drawn"] = seen["drawn"] || next.phase == PhaseDrawn
				seen["challenge"] = seen["challenge"] || next.phase == PhaseChallenge
				seen["reverse"] = seen["reverse"] || next.step != prev.step
			}
		}
	}
	for _, rule := range []string{"drawn", "challenge", "reverse"} {
		if !seen[rule] {
			t.Errorf("no bot game reached %s", rule)
		}
	}
}

// probes are every move the seat to act might try: each card it holds, a
// wild naming red, and every kind that takes no card.
func probes(v View) []Action {
	out := []Action{kind(ActionDraw), kind(ActionPass), kind(ActionChallenge), kind(ActionAccept), {Kind: ActionColor, Color: string(Red)}}
	for _, c := range v.MyHand {
		text := c.String()
		if c.Face.wild() {
			text += " r"
		}
		out = append(out, playing(text))
	}
	return out
}

// offers is whether v lets its seat make a.
func offers(t *testing.T, v View, a Action) bool {
	if !slices.Contains(v.Legal, a.Kind) {
		return false
	}
	if a.Kind != ActionPlay {
		return true
	}
	p, err := parsePlay(a.Card)
	if err != nil {
		t.Fatal(err)
	}
	return slices.Contains(v.Playable, p.card)
}

// TestViewLegalityIsWhatApplyAccepts holds the view to Apply: the seat to act
// may make a move exactly when Legal lists its kind and, for a play, Playable
// lists its card; every other seat is offered nothing.
func TestViewLegalityIsWhatApplyAccepts(t *testing.T) {
	for n := minPlayers; n <= maxPlayers; n++ {
		_, states := playToFinish(t, n, 5)
		for k, s := range states[:len(states)-1] {
			toAct := s.ToAct()[0]
			for _, at := range s.order {
				if v := s.View(at); at != toAct && (len(v.Legal) > 0 || len(v.Playable) > 0) {
					t.Fatalf("%d players step %d: %s is not to act but is offered %v %v", n, k, at, v.Legal, v.Playable)
				}
			}
			v := s.View(toAct)
			for _, a := range probes(v) {
				_, _, err := s.Apply(toAct, a, nil)
				if offered := offers(t, v, a); offered != (err == nil) {
					t.Fatalf("%d players step %d %s %+v: offered %t, Apply: %v; legal %v playable %v", n, k, toAct, a, offered, err, v.Legal, v.Playable)
				}
			}
		}
	}
}

// perturbed is s with one card of owner's hand swapped for the bottom card
// of the draw pile, which no step draws while the pile holds more than a
// step can take; false when the two cards are alike.
func perturbed(s state, owner int) (state, bool) {
	if len(s.pile) <= failedChallenge || len(s.hands[owner]) == 0 {
		return s, false
	}
	bottom := s.pile[len(s.pile)-1]
	at := slices.IndexFunc(s.hands[owner], func(c Card) bool { return c != bottom })
	if at < 0 {
		return s, false
	}
	s.hands = slices.Clone(s.hands)
	s.hands[owner] = slices.Clone(s.hands[owner])
	s.pile = slices.Clone(s.pile)
	s.hands[owner][at], s.pile[len(s.pile)-1] = bottom, s.hands[owner][at]
	return s, true
}

func viewsBut(t *testing.T, s state, except ...int) []string {
	t.Helper()
	out := []string{}
	for i, at := range s.order {
		if !slices.Contains(except, i) {
			out = append(out, encoded(t, s.View(at)))
		}
	}
	return out
}

// TestAHandReachesNoOtherSeat changes one seat's hidden hand and requires
// what every other seat is sent to stay byte for byte the same: its view
// now, the next step's events and errors, and its view after that step. The
// one thing a hand may change is the verdict of a challenge against it.
func TestAHandReachesNoOtherSeat(t *testing.T) {
	checked := 0
	for n := minPlayers; n <= maxPlayers; n++ {
		steps, states := playToFinish(t, n, game.Seed(20+n))
		for k, s := range states[:len(states)-1] {
			step := steps[k]
			actor := slices.Index(s.order, step.Seat)
			for owner := range s.order {
				changed, ok := perturbed(s, owner)
				if !ok {
					continue
				}
				if diff := cmp.Diff(viewsBut(t, s, owner), viewsBut(t, changed, owner)); diff != "" {
					t.Fatalf("%d players step %d: views depend on %s's hand:\n%s", n, k, s.order[owner], diff)
				}
				checked++
				if owner == actor || (step.Action.Kind == ActionChallenge && s.offender == owner) {
					continue
				}
				for _, probe := range probes(s.View(step.Seat)) {
					_, _, err := s.Apply(step.Seat, probe, nil)
					_, _, changedErr := changed.Apply(step.Seat, probe, nil)
					if fmt.Sprint(err) != fmt.Sprint(changedErr) {
						t.Fatalf("%d players step %d %+v: the error depends on %s's hand: %v / %v", n, k, probe, s.order[owner], err, changedErr)
					}
				}
				requireSameStep(t, s, changed, step, []int{owner}, fmt.Sprintf("%d players step %d: %s's hand", n, k, s.order[owner]))
			}
		}
	}
	if checked == 0 {
		t.Fatal("no hand was checked")
	}
}

// TestADrawnCardReachesOnlyItsDrawer turns the draw pile upside down, so a
// step draws other cards, and requires no event of a step, nor the view of any seat that drew
// nothing in it, to change.
func TestADrawnCardReachesOnlyItsDrawer(t *testing.T) {
	checked := 0
	for n := minPlayers; n <= maxPlayers; n++ {
		steps, states := playToFinish(t, n, game.Seed(40+n))
		for k, s := range states[:len(states)-1] {
			changed := s
			changed.pile = slices.Clone(s.pile)
			slices.Reverse(changed.pile)
			if len(s.pile) <= 2*failedChallenge || slices.Equal(s.pile[:failedChallenge], changed.pile[:failedChallenge]) {
				continue
			}
			next, _, _ := s.Apply(steps[k].Seat, steps[k].Action, nil)
			var drew []int
			for i, h := range next.(state).hands {
				if len(h) > len(s.hands[i]) {
					drew = append(drew, i)
				}
			}
			requireSameStep(t, s, changed, steps[k], drew, fmt.Sprintf("%d players step %d: the draw pile", n, k))
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no draw was checked")
	}
}

// requireSameStep fails when step from s and from changed makes different
// events, or different views after it for any seat but those in except.
func requireSameStep(t *testing.T, s, changed state, step gametest.Step[Action], except []int, what string) {
	t.Helper()
	next, events, _ := s.Apply(step.Seat, step.Action, nil)
	changedNext, changedEvents, _ := changed.Apply(step.Seat, step.Action, nil)
	if diff := cmp.Diff(encodedEvents(t, events), encodedEvents(t, changedEvents)); diff != "" {
		t.Fatalf("%s: events depend on it:\n%s", what, diff)
	}
	if diff := cmp.Diff(viewsBut(t, next.(state), except...), viewsBut(t, changedNext.(state), except...)); diff != "" {
		t.Fatalf("%s: views after the step depend on it:\n%s", what, diff)
	}
}

type persistedStep struct {
	seat   game.Seat
	action json.RawMessage
}

func TestRestoreReplaysPersistedStepsAndPlaysOnIdentically(t *testing.T) {
	const (
		seed      game.Seed = 11
		restoreAt game.Seq  = 25
		players             = 4
	)
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	start := func() game.ErasedState {
		st, _, err := g.New(opts, users(players)[0], users(players)[1:], game.StepRNG(seed, 0))
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
	if _, finished := live.Outcome(); finished {
		t.Fatalf("game finished before the restore point %d", restoreAt)
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
