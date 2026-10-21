package yahtzee

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/dice"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const maxGameSteps = 2000

func users(n int) []game.UserID { return []game.UserID{"alice", "bob", "carol", "dave"}[:n] }

func roll() Action           { return Action{Kind: ActionRoll} }
func hold(die int) Action    { return Action{Kind: ActionHold, Die: strconv.Itoa(die)} }
func scoreIn(box Box) Action { return Action{Kind: ActionScore, Box: string(box)} }
func seat(n int) game.Seat   { return game.NumberedSeat(n - 1) }
func newGame(t *testing.T, n int) state {
	t.Helper()
	st, _, err := Rules{}.New(Opts{}, users(n)[0], users(n)[1:], game.StepRNG(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	return st.(state)
}

func apply(t *testing.T, s state, by game.Seat, action Action, seq game.Seq) (state, []game.Event) {
	t.Helper()
	next, events, err := s.Apply(by, action, game.StepRNG(1, seq))
	if err != nil {
		t.Fatalf("%s %+v: %v", by, action, err)
	}
	return next.(state), events
}

// rolledTo is s with its seat to act having rolled once and shown h.
func rolledTo(s state, h hand) state {
	s.dice, s.rollsLeft = h, RollsPerTurn-1
	return s
}

// choose is a deterministic player: it rolls, keeps the most common face while
// the best box pays under 20, and scores the best-paying box, the first on ties.
func choose(v View) Action {
	if len(v.Dice) == 0 {
		return roll()
	}
	best := slices.MaxFunc(v.Options, func(a, b Option) int { return a.Score - b.Score })
	if !v.CanRoll || best.Score >= 20 {
		return scoreIn(best.Box)
	}
	keep := commonest(v.Dice)
	for i, face := range v.Dice {
		if v.Held[i] != (face == keep) {
			return hold(i + 1)
		}
	}
	return roll()
}

func commonest(faces []dice.Face) dice.Face {
	var counts [dice.Six + 1]int
	for _, f := range faces {
		counts[f]++
	}
	return dice.Face(slices.Index(counts[:], slices.Max(counts[:])))
}

// playToFinish plays choose for every seat with the rng each step gets in the
// server, and returns the steps and every state from the first to the final one.
func playToFinish(t *testing.T, n int, seed game.Seed) ([]gametest.Step[Action], []state) {
	t.Helper()
	st, _, err := Rules{}.New(Opts{}, users(n)[0], users(n)[1:], game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	states := []state{st.(state)}
	var steps []gametest.Step[Action]
	for seq := game.Seq(1); len(st.ToAct()) > 0; seq++ {
		if seq > maxGameSteps {
			t.Fatalf("%d players: no finish within %d steps", n, maxGameSteps)
		}
		by := st.ToAct()[0]
		action := choose(st.View(by))
		if st, _, err = st.Apply(by, action, game.StepRNG(seed, seq)); err != nil {
			t.Fatalf("seq %d %s %+v: %v", seq, by, action, err)
		}
		steps = append(steps, gametest.Step[Action]{Seat: by, Action: action})
		states = append(states, st.(state))
	}
	return steps, states
}

func TestNewSeatsOneToFourPlayers(t *testing.T) {
	for n := 1; n <= maxPlayers+1; n++ {
		all := []game.UserID{"alice", "bob", "carol", "dave", "erin"}[:n]
		_, seats, err := Rules{}.New(Opts{}, all[0], all[1:], nil)
		if (n <= maxPlayers) != (err == nil) {
			t.Errorf("%d players: err %v", n, err)
		}
		if err == nil && seats["alice"] != "p1" {
			t.Errorf("%d players: the challenger sits at %s, want p1", n, seats["alice"])
		}
	}
}

func TestApplyRejects(t *testing.T) {
	fresh := newGame(t, 2)
	rolled := rolledTo(fresh, hand{1, 2, 3, 4, 6})
	spent := rolled
	spent.rollsLeft = 0
	jokered := rolledTo(fresh, hand{3, 3, 3, 3, 3})
	jokered.cards = []card{cardWith(map[Box]int{BoxYahtzee: 50}), {}}
	allHeld := rolled
	allHeld.held = [DiceCount]bool{true, true, true, true, true}
	filled := rolled
	filled.cards = []card{cardWith(map[Box]int{BoxChance: 20}), {}}
	cases := []struct {
		name   string
		state  state
		by     game.Seat
		action Action
		want   string
	}{
		{"out of turn", fresh, seat(2), roll(), ""},
		{"unknown kind", fresh, seat(1), Action{Kind: "pass"}, "unknown action: pass"},
		{"hold before rolling", fresh, seat(1), hold(1), "roll first"},
		{"score before rolling", fresh, seat(1), scoreIn(BoxChance), "roll first"},
		{"a fourth roll", spent, seat(1), roll(), "no rolls left: score a box"},
		{"hold with no roll left", spent, seat(1), hold(2), "no rolls left: score a box"},
		{"die 0", rolled, seat(1), hold(0), `die must be 1 to 5, got "0"`},
		{"die 6", rolled, seat(1), hold(6), `die must be 1 to 5, got "6"`},
		{"a die that is no number", rolled, seat(1), Action{Kind: ActionHold, Die: "x"}, `die must be 1 to 5, got "x"`},
		{"a die with a leading zero", rolled, seat(1), Action{Kind: ActionHold, Die: "01"}, `die must be 1 to 5, got "01"`},
		{"a die with a sign", rolled, seat(1), Action{Kind: ActionHold, Die: "+1"}, `die must be 1 to 5, got "+1"`},
		{"a roll with every die held", allHeld, seat(1), roll(), "every die is held: release one or score a box"},
		{"no such box", rolled, seat(1), Action{Kind: ActionScore, Box: "pairs"}, `no box "pairs"`},
		{"a filled box", filled, seat(1), scoreIn(BoxChance), "chance is already scored"},
		{"a box the joker rule forbids", jokered, seat(1), scoreIn(BoxChance), "the joker rule allows only [threes]"},
	}
	for _, tc := range cases {
		_, _, err := tc.state.Apply(tc.by, tc.action, game.StepRNG(1, 1))
		var illegal game.IllegalActionError
		switch {
		case tc.want == "" && !errors.Is(err, game.ErrNotYourTurn):
			t.Errorf("%s: err %v, want not your turn", tc.name, err)
		case tc.want != "" && (!errors.As(err, &illegal) || illegal.Msg != tc.want):
			t.Errorf("%s: err %v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestHeldDiceKeepTheirFacesThroughARoll(t *testing.T) {
	s := rolledTo(newGame(t, 1), hand{6, 1, 6, 2, 3})
	s, _ = apply(t, s, seat(1), hold(1), 1)
	s, _ = apply(t, s, seat(1), hold(3), 2)
	s, _ = apply(t, s, seat(1), hold(5), 3)
	s, _ = apply(t, s, seat(1), hold(5), 4)
	if want := []bool{true, false, true, false, false}; !cmp.Equal(s.View(seat(1)).Held, want) {
		t.Fatalf("held %v after holding 1, 3 and 5 and releasing 5, want %v", s.View(seat(1)).Held, want)
	}
	after, events := apply(t, s, seat(1), roll(), 5)
	if after.dice[0] != 6 || after.dice[2] != 6 {
		t.Errorf("held sixes became %v", after.dice)
	}
	rerolled := dice.Roll(game.StepRNG(1, 5), 3, dice.Six)
	if got := []dice.Face{after.dice[1], after.dice[3], after.dice[4]}; !cmp.Equal(got, rerolled) {
		t.Errorf("free dice rolled %v, want the step's rng in order, %v", got, rerolled)
	}
	want := []game.Event{RolledEvent{By: seat(1), Dice: after.dice[:]}}
	if diff := cmp.Diff(want, events); diff != "" {
		t.Errorf("events (-want +got):\n%s", diff)
	}
	if after.rollsLeft != 1 || after.View(seat(1)).CanRoll != true {
		t.Errorf("after two rolls: rolls left %d", after.rollsLeft)
	}
}

func TestScoringPassesTheTurnWithFreshDice(t *testing.T) {
	s := rolledTo(newGame(t, 2), hand{2, 2, 2, 2, 2})
	s.held[0] = true
	s.cards = []card{cardWith(map[Box]int{BoxYahtzee: 50}), {}}
	next, events := apply(t, s, seat(1), scoreIn(BoxTwos), 1)
	want := []game.Event{ScoredEvent{By: seat(1), Box: BoxTwos, Score: 10, YahtzeeBonus: 100}}
	if diff := cmp.Diff(want, events); diff != "" {
		t.Errorf("events (-want +got):\n%s", diff)
	}
	if !cmp.Equal(next.ToAct(), []game.Seat{seat(2)}) {
		t.Errorf("to act %v, want p2", next.ToAct())
	}
	v := next.View(seat(2))
	if len(v.Dice) != 0 || slices.Contains(v.Held, true) || v.RollsLeft != RollsPerTurn || !v.CanRoll || v.CanHold || len(v.Options) != 0 {
		t.Errorf("p2's fresh turn: %+v", v)
	}
	if c := v.Cards[0]; c.Total != 160 || c.YahtzeeBonus != 100 || !cmp.Equal(c.Scores, map[Box]int{BoxYahtzee: 50, BoxTwos: 10}) {
		t.Errorf("p1's card %+v, want yahtzee 50, twos 10, bonus 100, total 160", c)
	}
}

func TestOnlyTheSeatToActIsOfferedAnything(t *testing.T) {
	s := rolledTo(newGame(t, 3), hand{1, 1, 4, 5, 6})
	for _, viewer := range []game.Seat{seat(2), seat(3)} {
		if v := s.View(viewer); v.CanRoll || v.CanHold || len(v.Options) != 0 {
			t.Errorf("%s waits but is offered %+v", viewer, v)
		}
	}
	if v := s.View(seat(1)); !v.CanRoll || !v.CanHold || len(v.Options) != boxCount {
		t.Errorf("p1 to act: %+v", v)
	}
}

func TestRollIsNotOfferedWithEveryDieHeld(t *testing.T) {
	s := rolledTo(newGame(t, 1), hand{1, 2, 3, 4, 6})
	s.held = [DiceCount]bool{true, true, true, true, true}
	if v := s.View(seat(1)); v.CanRoll || !v.CanHold {
		t.Errorf("every die held: can_roll %t, can_hold %t; want no roll, holds still open", v.CanRoll, v.CanHold)
	}
}

func TestViewLegalityIsWhatApplyAccepts(t *testing.T) {
	for n := 1; n <= maxPlayers; n++ {
		_, states := playToFinish(t, n, game.Seed(n))
		for _, s := range states[:len(states)-1] {
			by := s.ToAct()[0]
			v := s.View(by)
			_, _, err := s.Apply(by, roll(), game.StepRNG(1, 1))
			if v.CanRoll != (err == nil) {
				t.Fatalf("can_roll %t, roll: %v", v.CanRoll, err)
			}
			_, _, err = s.Apply(by, hold(1), nil)
			if v.CanHold != (err == nil) {
				t.Fatalf("can_hold %t, hold: %v", v.CanHold, err)
			}
			offered := optionBoxes(v.Options)
			for _, box := range Boxes() {
				next, _, err := s.Apply(by, scoreIn(box), nil)
				points, ok := offered[box]
				if ok != (err == nil) {
					t.Fatalf("%s offered %t, score: %v", box, ok, err)
				}
				if ok {
					got := next.(state).cards[s.turn]
					if i, _ := boxIndex(box); got.points[i] != points {
						t.Fatalf("%s offered at %d, scored %d", box, points, got.points[i])
					}
				}
			}
		}
	}
}

func TestFullCardsEndTheGame(t *testing.T) {
	full := func(chance int) card {
		points := map[Box]int{}
		for _, box := range Boxes() {
			points[box] = 0
		}
		points[BoxChance] = chance
		return cardWith(points)
	}
	left := func(c card) card {
		c.out = true
		return c
	}
	lastTurn := func(cards []card, last game.Seat) state {
		s := newGame(t, len(cards))
		s.cards = slices.Clone(cards)
		s.turn = slices.Index(s.order, last)
		i, _ := boxIndex(BoxChance)
		s.cards[s.turn].filled[i], s.cards[s.turn].points[i] = false, 0
		return rolledTo(s, hand{1, 1, 1, 2, 2})
	}
	cases := []struct {
		name  string
		state state
		want  game.Result
	}{
		{"solo", lastTurn([]card{full(7)}, seat(1)), game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat(1)}, Reason: ReasonScorecardsFull}},
		{"the highest total wins", lastTurn([]card{full(20), full(7)}, seat(2)), game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat(1)}, Reason: ReasonScorecardsFull}},
		{"equal totals draw", lastTurn([]card{full(7), full(7)}, seat(2)), game.Result{Kind: game.ResultDraw, Reason: ReasonScorecardsFull}},
		{"two of three tied at the top both win", lastTurn([]card{full(7), full(5), full(7)}, seat(3)), game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat(1), seat(3)}, Reason: ReasonScorecardsFull}},
		{"a tie among those left after a leaver is a win for each", lastTurn([]card{full(7), left(full(9)), full(7)}, seat(3)), game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat(1), seat(3)}, Reason: ReasonScorecardsFull}},
	}
	for _, tc := range cases {
		end, _ := apply(t, tc.state, tc.state.ToAct()[0], scoreIn(BoxChance), 1)
		got, finished := end.Outcome()
		if diff := cmp.Diff(tc.want, got); !finished || diff != "" {
			t.Errorf("%s: finished %t (-want +got):\n%s", tc.name, finished, diff)
		}
		if v := end.View(seat(1)); v.CanRoll || len(v.Options) != 0 || v.Round != Rounds {
			t.Errorf("%s: the final view offers %+v", tc.name, v)
		}
	}
}

func TestAFullGameIsThirteenRoundsAndTheBestCardWins(t *testing.T) {
	for n := 1; n <= maxPlayers; n++ {
		steps, states := playToFinish(t, n, 7)
		final := states[len(states)-1]
		scored := slices.DeleteFunc(slices.Clone(steps), func(s gametest.Step[Action]) bool { return s.Action.Kind != ActionScore })
		if len(scored) != Rounds*n {
			t.Errorf("%d players: %d boxes scored, want %d", n, len(scored), Rounds*n)
		}
		totals := map[game.Seat]int{}
		for i, c := range final.cards {
			if !c.full() {
				t.Errorf("%d players: %s's card is not full", n, final.order[i])
			}
			totals[final.order[i]] = c.total()
		}
		best := slices.Max(slices.Collect(maps.Values(totals)))
		result, _ := final.Outcome()
		for _, winner := range result.Winners {
			if totals[winner] != best {
				t.Errorf("%d players: winner %s totals %d, best %d", n, winner, totals[winner], best)
			}
		}
	}
}

// D9: the core scores the result, +1 a win and -1 a loss; card totals stay in the view.
func TestTheCoreScoresTheResult(t *testing.T) {
	if _, scored := any(newGame(t, 2)).(game.Scorer); scored {
		t.Fatal("yahtzee is a game.Scorer; D9 wants the core's result scoring")
	}
}

func TestEliminate(t *testing.T) {
	solo, _ := newGame(t, 1).Eliminate(seat(1), game.ReasonResign)
	if got, finished := solo.Outcome(); !finished || got.Kind != game.ResultDecisive || len(got.Winners) != 0 {
		t.Errorf("solo give-up: %+v, finished %t; want a loss", got, finished)
	}
	duel, _ := newGame(t, 2).Eliminate(seat(1), game.ReasonResign)
	if got, _ := duel.Outcome(); !cmp.Equal(got, game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat(2)}, Reason: game.ReasonResign}) {
		t.Errorf("heads-up resign: %+v", got)
	}

	three := rolledTo(newGame(t, 3), hand{1, 2, 3, 4, 5})
	left, events := three.Eliminate(seat(1), game.ReasonResign)
	if _, finished := left.Outcome(); finished || len(events) != 0 || !cmp.Equal(left.ToAct(), []game.Seat{seat(2)}) {
		t.Fatalf("p1 leaves on its turn of three: finished %t, to act %v", finished, left.ToAct())
	}
	if v := left.View(seat(2)); len(v.Dice) != 0 || !v.Cards[0].Out {
		t.Errorf("p2 takes a fresh turn and p1's card stays, out: %+v", v)
	}
	waiting, _ := three.Eliminate(seat(3), game.ReasonResign)
	if !cmp.Equal(waiting.ToAct(), []game.Seat{seat(1)}) || waiting.(state).dice != three.dice {
		t.Errorf("a waiting seat leaving keeps p1's turn and dice")
	}
	s := waiting.(state)
	s, _ = apply(t, s, seat(1), scoreIn(BoxLargeStraight), 1)
	if !cmp.Equal(s.ToAct(), []game.Seat{seat(2)}) {
		t.Fatalf("after p1 scores, to act %v, want p2", s.ToAct())
	}
	s = rolledTo(s, hand{1, 1, 1, 1, 2})
	s, _ = apply(t, s, seat(2), scoreIn(BoxOnes), 2)
	if !cmp.Equal(s.ToAct(), []game.Seat{seat(1)}) {
		t.Errorf("the turn skips p3, who left: to act %v", s.ToAct())
	}
}

func TestConcludeOffersNothing(t *testing.T) {
	s := rolledTo(newGame(t, 2), hand{1, 2, 3, 4, 5})
	imposed := game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	done := s.Conclude(imposed)
	if got, finished := done.Outcome(); !finished || !cmp.Equal(got, imposed) || len(done.ToAct()) != 0 {
		t.Fatalf("concluded: %+v, finished %t", got, finished)
	}
	if v := done.View(seat(1)); v.CanRoll || v.CanHold || len(v.Options) != 0 {
		t.Errorf("concluded view offers %+v", v)
	}
	if again := done.(state).Conclude(game.Result{Kind: game.ResultDraw}); !cmp.Equal(again, done, cmp.AllowUnexported(state{}, card{})) {
		t.Errorf("a second conclusion replaced the first")
	}
}

func TestApplyLeavesTheEarlierStateUnchanged(t *testing.T) {
	s := rolledTo(newGame(t, 2), hand{3, 3, 3, 4, 4})
	before := s.View(seat(1))
	for _, action := range []Action{roll(), hold(2), scoreIn(BoxFullHouse)} {
		apply(t, s, seat(1), action, 1)
		if diff := cmp.Diff(before, s.View(seat(1))); diff != "" {
			t.Errorf("%+v changed the earlier state (-before +after):\n%s", action, diff)
		}
	}
}

func TestWireShape(t *testing.T) {
	raw, err := json.Marshal(newGame(t, 1).View(seat(1)))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"round":1,"dice":[],"held":[false,false,false,false,false],"rolls_left":3,` +
		`"cards":[{"seat":"p1","scores":{},"upper_total":0,"upper_bonus":0,"yahtzee_bonus":0,"total":0,"out":false}],` +
		`"can_roll":true,"can_hold":false,"options":[]}`
	if string(raw) != want {
		t.Errorf("view\n got %s\nwant %s", raw, want)
	}
}

func TestConformance(t *testing.T) {
	var scripts []gametest.Script[Opts, Action]
	for n := 1; n <= maxPlayers; n++ {
		for _, seed := range []game.Seed{3, 8} {
			steps, states := playToFinish(t, n, seed)
			illegal := []gametest.Step[Action]{
				{Seat: seat(1), Action: scoreIn(BoxChance), WantErr: true},
				{Seat: seat(1), Action: hold(1), WantErr: true},
			}
			if n > 1 {
				illegal = append(illegal, gametest.Step[Action]{Seat: seat(2), Action: roll(), WantErr: true})
			}
			result, _ := states[len(states)-1].Outcome()
			scripts = append(scripts, gametest.Script[Opts, Action]{
				Name:       fmt.Sprintf("%d players seed %d", n, seed),
				Challenger: users(n)[0],
				Invitees:   users(n)[1:],
				Seed:       seed,
				Steps:      append(illegal, steps...),
				WantResult: &result,
			})
		}
		steps, _ := playToFinish(t, n, 5)
		scripts = append(scripts, gametest.Script[Opts, Action]{
			Name: fmt.Sprintf("%d players unfinished", n), Challenger: users(n)[0], Invitees: users(n)[1:], Seed: 5,
			Steps: steps[:len(steps)/2],
		})
	}
	gametest.Run(t, Rules{}, scripts)
}

type persistedStep struct {
	seat   game.Seat
	action json.RawMessage
}

func TestRestoreReplaysPersistedStepsAndPlaysOnIdentically(t *testing.T) {
	const (
		seed      game.Seed = 11
		restoreAt game.Seq  = 60
		players             = 3
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
			break
		}
		step := next(live)
		var liveEvents, restoredEvents []json.RawMessage
		live, liveEvents = apply(live, step, seq)
		restored, restoredEvents = apply(restored, step, seq)
		if diff := cmp.Diff(liveEvents, restoredEvents); diff != "" {
			t.Fatalf("seq %d: restored events differ (-live +restored):\n%s", seq, diff)
		}
	}
	liveResult, _ := live.Outcome()
	restoredResult, finished := restored.Outcome()
	if diff := cmp.Diff(liveResult, restoredResult); !finished || diff != "" {
		t.Errorf("restored finished=%t, result (-live +restored):\n%s", finished, diff)
	}
}
