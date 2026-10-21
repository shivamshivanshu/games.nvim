package liarsdice

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/dice"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const maxGameSteps = 5000

var playerRange = Rules{}.Info().Players

func seat(n int) game.Seat { return game.NumberedSeat(n - 1) }

func bid(text string) Action { return Action{Kind: ActionBid, Bid: text} }

func dudo() Action { return Action{Kind: ActionDudo} }

func newGame(t *testing.T, n int, seed game.Seed) state {
	t.Helper()
	users := gametest.Users(n)
	st, _, err := Rules{}.New(Opts{}, users[0], users[1:], game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	return st.(state)
}

// withCups is s with these cups on the table, p1 to act and no bid.
func withCups(s state, cups ...[]dice.Face) state {
	s.cups, s.turn, s.bid = cups, 0, nil
	return s
}

func apply(t *testing.T, s state, by game.Seat, action Action, seq game.Seq) (state, []game.Event) {
	t.Helper()
	return gametest.MustApply(t, s, by, action, game.StepRNG(1, seq))
}

// choose is a deterministic player: it calls dudo on a bid above what its own
// dice and a third of the others' make likely, and otherwise makes the least
// bid on the face it holds most of.
func choose(v View) Action {
	if v.Bid != nil && v.CanDudo && (len(v.MinBids) == 0 || v.Bid.Quantity > likely(v, v.Bid.Face)+1) {
		return dudo()
	}
	best := slices.MaxFunc(v.MinBids, func(a, b BidMinimum) int {
		return (likely(v, a.Face) - a.Quantity) - (likely(v, b.Face) - b.Quantity)
	})
	return bid(fmt.Sprintf("%dx%d", best.Quantity, best.Face))
}

func likely(v View, face dice.Face) int {
	mine := 0
	for _, f := range v.MyDice {
		if f == face || f == Wild {
			mine++
		}
	}
	others := v.TotalDice - len(v.MyDice)
	if face == Wild {
		return mine + others/6
	}
	return mine + others/3
}

// playToFinish plays choose for every seat of n from seed, and returns the
// steps and every state from the first to the final one.
func playToFinish(t *testing.T, n int, seed game.Seed) ([]gametest.Step[Action], []state) {
	t.Helper()
	steps := gametest.PlayToFinish(t, Rules{}, Opts{}, gametest.Users(n), seed, func(v View, _ game.Seat) Action { return choose(v) }, maxGameSteps)
	return steps, gametest.Replay(t, newGame(t, n, seed), steps, seed)
}

func TestNewSeatsTwoToSixPlayersWithFiveDiceEach(t *testing.T) {
	for n := playerRange.Min; n <= playerRange.Max; n++ {
		users := gametest.Users(n)
		st, seats, err := Rules{}.New(Opts{}, users[0], users[1:], game.StepRNG(1, 0))
		if err != nil {
			t.Fatalf("%d players: %v", n, err)
		}
		s := st.(state)
		if seats["alice"] != seat(1) || !cmp.Equal(s.ToAct(), []game.Seat{seat(1)}) || s.totalDice() != n*StartingDice {
			t.Errorf("%d players: seats %v, to act %v, %d dice", n, seats, s.ToAct(), s.totalDice())
		}
		for i, cup := range s.cups {
			if want := dice.Roll(game.StepRNG(1, 0), n*StartingDice, dice.Six)[i*StartingDice : (i+1)*StartingDice]; !cmp.Equal(cup, sorted(want)) {
				t.Errorf("%d players: cup %d = %v, want the seed's rolls %v sorted", n, i, cup, want)
			}
		}
	}
}

func sorted(faces []dice.Face) []dice.Face {
	out := slices.Clone(faces)
	slices.Sort(out)
	return out
}

func TestApplyRejects(t *testing.T) {
	fresh := withCups(newGame(t, 3, 1), []dice.Face{1, 2, 3}, []dice.Face{4, 5}, []dice.Face{6})
	raised := fresh
	raised.bid = &Bid{By: seat(3), Quantity: 2, Face: 4}
	cases := []struct {
		name   string
		state  state
		by     game.Seat
		action Action
		want   string
	}{
		{"unknown kind", fresh, seat(1), Action{Kind: "calza"}, "unknown action: calza"},
		{"not a bid", fresh, seat(1), bid("3 fives"), `a bid is quantity x face, e.g. 3x5, got "3 fives"`},
		{"opening on ones", fresh, seat(1), bid("1x1"), "the opening bid cannot be on ones"},
		{"no dice", fresh, seat(1), bid("0x3"), "a bid on 3s needs at least 1 dice"},
		{"more dice than in play", fresh, seat(1), bid("7x3"), "only 6 dice are in play"},
		{"dudo with no bid", fresh, seat(1), dudo(), "there is no bid to challenge: open the bidding"},
		{"no raise", raised, seat(1), bid("2x4"), "a bid on 4s needs at least 3 dice"},
		{"a lower face at the same quantity", raised, seat(1), bid("2x3"), "a bid on 3s needs at least 3 dice"},
		{"too few ones", raised, seat(1), bid("0x1"), "a bid on 1s needs at least 1 dice"},
	}
	for _, tc := range cases {
		_, _, err := tc.state.Apply(tc.by, tc.action, game.StepRNG(1, 1))
		if illegal, ok := errors.AsType[game.IllegalActionError](err); !ok || illegal.Msg != tc.want {
			t.Errorf("%s: err %v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestABidPassesTheTurnToTheNextSeatWithDice(t *testing.T) {
	s := withCups(newGame(t, 3, 1), []dice.Face{2, 2}, nil, []dice.Face{3})
	next, events := apply(t, s, seat(1), bid("2x2"), 1)
	if !cmp.Equal(next.ToAct(), []game.Seat{seat(3)}) {
		t.Errorf("to act %v, want p3: p2 is out", next.ToAct())
	}
	if diff := cmp.Diff([]game.Event{BidEvent{By: seat(1), Quantity: 2, Face: 2}}, events); diff != "" {
		t.Errorf("events (-want +got):\n%s", diff)
	}
	if v := next.View(seat(3)); !cmp.Equal(v.Bid, &Bid{By: seat(1), Quantity: 2, Face: 2}) || !v.CanDudo {
		t.Errorf("p3's view: bid %+v, can dudo %t", v.Bid, v.CanDudo)
	}
}

func TestDudo(t *testing.T) {
	table := func() state {
		return withCups(newGame(t, 3, 1), []dice.Face{1, 4, 4}, []dice.Face{2, 4}, []dice.Face{1, 6})
	}
	cases := []struct {
		name      string
		bid       Bid
		challenge game.Seat
		counted   int
		loser     game.Seat
	}{
		{"a bid short of the count loses the bidder a die", Bid{By: seat(3), Quantity: 6, Face: 4}, seat(1), 5, seat(3)},
		{"a bid met exactly loses the challenger a die", Bid{By: seat(3), Quantity: 5, Face: 4}, seat(1), 5, seat(1)},
		{"a bid on ones counts only ones", Bid{By: seat(2), Quantity: 3, Face: Wild}, seat(3), 2, seat(2)},
	}
	for _, tc := range cases {
		s := table()
		s.bid, s.turn = &tc.bid, slices.Index(s.order, tc.challenge)
		next, events := apply(t, s, tc.challenge, dudo(), 9)
		want := Reveal{Round: 1, Bid: tc.bid, Challenger: tc.challenge, Counted: tc.counted, Loser: tc.loser, Cups: s.seatDice()}
		wantEvents := []game.Event{ChallengedEvent{By: tc.challenge, Bid: tc.bid, Counted: tc.counted, Loser: tc.loser, Cups: s.seatDice()}}
		if diff := cmp.Diff(wantEvents, events); diff != "" {
			t.Errorf("%s: events (-want +got):\n%s", tc.name, diff)
		}
		v := next.View(seat(1))
		if diff := cmp.Diff(&want, v.LastReveal); diff != "" {
			t.Errorf("%s: reveal (-want +got):\n%s", tc.name, diff)
		}
		loser := slices.Index(s.order, tc.loser)
		if len(next.cups[loser]) != len(s.cups[loser])-1 || next.totalDice() != s.totalDice()-1 {
			t.Errorf("%s: the loser has %d dice, %d in play", tc.name, len(next.cups[loser]), next.totalDice())
		}
		if v.Round != 2 || v.Bid != nil || !cmp.Equal(next.ToAct(), []game.Seat{tc.loser}) {
			t.Errorf("%s: round %d, bid %+v, to act %v; want round 2 opened by the loser", tc.name, v.Round, v.Bid, next.ToAct())
		}
		sizes := []int{len(next.cups[0]), len(next.cups[1]), len(next.cups[2])}
		rolled := dice.Roll(game.StepRNG(1, 9), next.totalDice(), dice.Six)
		if got := slices.Concat(next.cups...); !cmp.Equal(got, slices.Concat(sorted(rolled[:sizes[0]]), sorted(rolled[sizes[0]:sizes[0]+sizes[1]]), sorted(rolled[sizes[0]+sizes[1]:]))) {
			t.Errorf("%s: cups %v are not the step's reroll %v", tc.name, next.cups, rolled)
		}
	}
}

func TestLosingTheLastDieSeatsTheNextOpenerOrEndsTheGame(t *testing.T) {
	s := withCups(newGame(t, 3, 1), []dice.Face{5}, []dice.Face{2, 3}, []dice.Face{4, 6})
	s.bid = &Bid{By: seat(1), Quantity: 2, Face: 5}
	s.turn = 1
	next, events := apply(t, s, seat(2), dudo(), 1)
	if len(events) != 2 || events[1] != (OutEvent{Seat: seat(1)}) {
		t.Fatalf("events %+v, want the reveal and p1 out", events)
	}
	if !cmp.Equal(next.ToAct(), []game.Seat{seat(2)}) || !next.View(seat(1)).Seats[0].Out || len(next.View(seat(1)).MyDice) != 0 {
		t.Errorf("p1 is out of dice: to act %v, seats %+v", next.ToAct(), next.View(seat(1)).Seats)
	}
	if got := next.KnockedOut(); !cmp.Equal(got, []game.Seat{seat(1)}) {
		t.Errorf("knocked out %v, want p1, so the core takes its controls", got)
	}

	last := withCups(newGame(t, 3, 1), nil, []dice.Face{2}, []dice.Face{4, 6})
	last.bid, last.turn = &Bid{By: seat(3), Quantity: 1, Face: 2}, 1
	end, _ := apply(t, last, seat(2), dudo(), 1)
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat(3)}, Reason: ReasonLastWithDice}
	if got, finished := end.Outcome(); !finished || !cmp.Equal(got, want) {
		t.Errorf("p2 loses its last die: %+v, finished %t", got, finished)
	}
	if v := end.View(seat(3)); v.CanDudo || len(v.MinBids) != 0 {
		t.Errorf("the final view offers %+v", v)
	}
}

func TestOnlyTheSeatToActIsOfferedBids(t *testing.T) {
	s := withCups(newGame(t, 2, 1), []dice.Face{3, 3}, []dice.Face{1, 6})
	s.bid = &Bid{By: seat(2), Quantity: 1, Face: 6}
	want := []BidMinimum{{Face: 1, Quantity: 1}, {Face: 2, Quantity: 2}, {Face: 3, Quantity: 2}, {Face: 4, Quantity: 2}, {Face: 5, Quantity: 2}, {Face: 6, Quantity: 2}}
	if v := s.View(seat(1)); !cmp.Equal(v.MinBids, want) || !v.CanDudo {
		t.Errorf("p1 to act: min bids %+v, can dudo %t", v.MinBids, v.CanDudo)
	}
	if v := s.View(seat(2)); len(v.MinBids) != 0 || v.CanDudo {
		t.Errorf("p2 waits but is offered %+v, dudo %t", v.MinBids, v.CanDudo)
	}
	s.bid = &Bid{By: seat(2), Quantity: 2, Face: Wild}
	if v := s.View(seat(1)); !cmp.Equal(v.MinBids, []BidMinimum{{Face: Wild, Quantity: 3}}) {
		t.Errorf("after 2x1 with 4 dice in play, min bids %+v; want only 3x1", v.MinBids)
	}
}

// TestViewLegalityIsWhatApplyAccepts holds the view to Apply: each face's
// minimum and the dice in play bound the bids Apply takes, dudo is legal
// exactly when offered, and no other seat is offered anything.
func TestViewLegalityIsWhatApplyAccepts(t *testing.T) {
	for n := playerRange.Min; n <= playerRange.Max; n++ {
		_, states := playToFinish(t, n, game.Seed(n))
		for _, s := range states[:len(states)-1] {
			by := s.ToAct()[0]
			v := s.View(by)
			for _, other := range s.order {
				if ov := s.View(other); other != by && (len(ov.MinBids) > 0 || ov.CanDudo) {
					t.Fatalf("%s waits but is offered %+v", other, ov.MinBids)
				}
			}
			if _, _, err := s.Apply(by, dudo(), game.StepRNG(1, 1)); v.CanDudo != (err == nil) {
				t.Fatalf("can_dudo %t, dudo: %v", v.CanDudo, err)
			}
			for face := dice.Face(1); face <= dice.Six; face++ {
				least := slices.IndexFunc(v.MinBids, func(m BidMinimum) bool { return m.Face == face })
				for q := 0; q <= v.TotalDice+1; q++ {
					_, _, err := s.Apply(by, bid(fmt.Sprintf("%dx%d", q, face)), nil)
					offered := least >= 0 && q >= v.MinBids[least].Quantity && q <= v.TotalDice
					if offered != (err == nil) {
						t.Fatalf("%dx%d after %+v: offered %t, Apply: %v", q, face, v.Bid, offered, err)
					}
				}
			}
		}
	}
}

// otherDice is s with the dice of seat owner changed, each face moved up by
// one, and owner as the one seat that may see it; none if owner has no dice.
func otherDice(st game.State[Action, View], owner game.Seat) (game.State[Action, View], []game.Seat) {
	s := st.(state)
	i := slices.Index(s.order, owner)
	if len(s.cups[i]) == 0 {
		return s, nil
	}
	s.cups = slices.Clone(s.cups)
	s.cups[i] = slices.Clone(s.cups[i])
	for k, f := range s.cups[i] {
		s.cups[i][k] = f%dice.Six + 1
	}
	return s, []game.Seat{owner}
}

// dudoTellsEverySeat reveals every cup.
func dudoTellsEverySeat(st game.State[Action, View], step gametest.Step[Action], _ []game.Seat) []game.Seat {
	if step.Action.Kind != ActionDudo {
		return nil
	}
	return st.(state).order
}

// TestASeatsDiceReachNoOtherSeatUntilRevealed changes one seat's hidden dice
// and requires what every other seat is sent to stay byte for byte the same
// until a dudo reveals them.
func TestASeatsDiceReachNoOtherSeatUntilRevealed(t *testing.T) {
	for n := playerRange.Min; n <= playerRange.Max; n++ {
		seed := game.Seed(100 + n)
		steps, _ := playToFinish(t, n, seed)
		users := gametest.Users(n)
		script := gametest.Script[Opts, Action]{Challenger: users[0], Invitees: users[1:], Seed: seed, Steps: steps}
		gametest.NonInterference(t, Rules{}, script, gametest.Hidden[Action, View]{Perturb: otherDice, Reveals: dudoTellsEverySeat})
	}
}

func TestEliminate(t *testing.T) {
	duel, _ := newGame(t, 2, 1).Eliminate(seat(1), game.ReasonResign)
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat(2)}, Reason: game.ReasonResign}
	if got, finished := duel.Outcome(); !finished || !cmp.Equal(got, want) {
		t.Errorf("heads-up resign: %+v, finished %t", got, finished)
	}

	s := withCups(newGame(t, 3, 1), []dice.Face{1, 2}, []dice.Face{3, 4}, []dice.Face{5, 6})
	s.bid, s.turn = &Bid{By: seat(1), Quantity: 2, Face: 3}, 1
	cases := []struct {
		name    string
		leaver  game.Seat
		toAct   game.Seat
		inPlay  int
		myDice  []dice.Face
		watcher game.Seat
	}{
		{"the seat to act leaves: the next seat opens", seat(2), seat(3), 4, []dice.Face{5, 6}, seat(3)},
		{"the bidder leaves: the seat to act opens", seat(1), seat(2), 4, []dice.Face{3, 4}, seat(2)},
	}
	for _, tc := range cases {
		left, events := s.Eliminate(tc.leaver, game.ReasonResign)
		if _, finished := left.Outcome(); finished || len(events) != 0 || !cmp.Equal(left.ToAct(), []game.Seat{tc.toAct}) {
			t.Errorf("%s: finished %t, to act %v", tc.name, finished, left.ToAct())
		}
		v := left.View(tc.watcher)
		if v.Bid != nil || v.TotalDice != tc.inPlay || !cmp.Equal(v.MyDice, tc.myDice) || v.Round != 1 {
			t.Errorf("%s: bid %+v, %d dice, mine %v, round %d; want the bidding void on the same dice", tc.name, v.Bid, v.TotalDice, v.MyDice, v.Round)
		}
	}
	once, _ := s.Eliminate(seat(2), game.ReasonResign)
	if raw := gametest.ViewJSON(t, once.View(seat(2)).MyDice); raw != "[]" {
		t.Errorf("the leaver's dice encode as %s, want []", raw)
	}
	twice, events := once.Eliminate(seat(2), game.ReasonResign)
	if len(events) != 0 || !cmp.Equal(once, twice, cmp.AllowUnexported(state{}, game.Ending{})) {
		t.Errorf("eliminating a seat with no dice is not a no-op")
	}
}

func TestConcludeOffersNothing(t *testing.T) {
	s := newGame(t, 2, 1)
	s.bid, s.turn = &Bid{By: seat(1), Quantity: 1, Face: 3}, 1
	imposed := game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	done := s.Conclude(imposed)
	if got, finished := done.Outcome(); !finished || !cmp.Equal(got, imposed) || len(done.ToAct()) != 0 {
		t.Fatalf("concluded: %+v, finished %t", got, finished)
	}
	if v := done.View(seat(2)); v.CanDudo || len(v.MinBids) != 0 {
		t.Errorf("concluded view offers %+v", v)
	}
}

func TestApplyLeavesTheEarlierStateUnchanged(t *testing.T) {
	s := newGame(t, 3, 1)
	s.bid, s.turn = &Bid{By: seat(3), Quantity: 2, Face: 3}, 0
	before := map[game.Seat]string{}
	for _, seat := range s.order {
		before[seat] = gametest.ViewJSON(t, s.View(seat))
	}
	for _, action := range []Action{bid("3x3"), dudo()} {
		next, _ := apply(t, s, seat(1), action, 1)
		mutated := next.View(seat(1))
		mutated.MyDice[0] = 9
		if mutated.LastReveal != nil {
			mutated.LastReveal.Cups[0].Dice[0] = 9
		}
		for _, seat := range s.order {
			if got := gametest.ViewJSON(t, s.View(seat)); got != before[seat] {
				t.Errorf("%+v changed %s's earlier view:\n%s\n%s", action, seat, before[seat], got)
			}
		}
	}
}

func TestWireShape(t *testing.T) {
	s := withCups(newGame(t, 2, 1), []dice.Face{2, 5}, []dice.Face{1, 3, 6})
	raw := gametest.ViewJSON(t, s.View(seat(2)))
	want := `{"round":1,"seats":[{"seat":"p1","dice":2,"out":false},{"seat":"p2","dice":3,"out":false}],` +
		`"bid":null,"total_dice":5,"my_dice":[1,3,6],"min_bids":[],"can_dudo":false,"last_reveal":null}`
	if raw != want {
		t.Errorf("view\n got %s\nwant %s", raw, want)
	}
}

func TestCataloguePatternIsTheParsersOwn(t *testing.T) {
	if got := (Rules{}).Info().Actions[0].Input.Pattern; got != bidSyntax.String() {
		t.Errorf("catalogue pattern %q, parser %q", got, bidSyntax.String())
	}
}

func TestConformance(t *testing.T) {
	var scripts []gametest.Script[Opts, Action]
	for n := playerRange.Min; n <= playerRange.Max; n++ {
		users := gametest.Users(n)
		for _, seed := range []game.Seed{3, 8} {
			steps, states := playToFinish(t, n, seed)
			illegal := []gametest.Step[Action]{
				{Seat: seat(2), Action: bid("1x2"), WantErr: true},
				{Seat: seat(1), Action: dudo(), WantErr: true},
				{Seat: seat(1), Action: bid("1x1"), WantErr: true},
			}
			result, _ := states[len(states)-1].Outcome()
			scripts = append(scripts, gametest.Script[Opts, Action]{
				Name:       fmt.Sprintf("%d players seed %d", n, seed),
				Challenger: users[0],
				Invitees:   users[1:],
				Seed:       seed,
				Steps:      append(illegal, steps...),
				WantResult: &game.Result{Kind: game.ResultDecisive, Winners: result.Winners, Reason: ReasonLastWithDice},
			})
		}
	}
	gametest.Run(t, Rules{}, scripts)
}
