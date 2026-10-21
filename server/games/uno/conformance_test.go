package uno

import (
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

const maxGameSteps = 5000

var playerRange = Rules{}.Info().Players

// playToFinish plays choose for every seat of n from seed, and returns the
// steps and every state from the first to the final one.
func playToFinish(t *testing.T, n int, seed game.Seed) ([]gametest.Step[Action], []state) {
	t.Helper()
	steps := gametest.PlayToFinish(t, Rules{}, Opts{}, gametest.Users(n), seed, func(v View, _ game.Seat) Action { return choose(v) }, maxGameSteps)
	return steps, gametest.Replay(t, Rules{}.Info(), newGame(t, n, seed), steps, seed)
}

func TestConformance(t *testing.T) {
	var scripts []gametest.Script[Opts, Action]
	for n := playerRange.Min; n <= playerRange.Max; n++ {
		users := gametest.Users(n)
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
				Challenger: users[0],
				Invitees:   users[1:],
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
	for n := playerRange.Min; n <= playerRange.Max; n++ {
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
	for n := playerRange.Min; n <= playerRange.Max; n++ {
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

// sameCards is whether a and b hold the same cards, in any order.
func sameCards(a, b []Card) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.SortFunc(a, cardOrder)
	slices.SortFunc(b, cardOrder)
	return slices.Equal(a, b)
}

// perturbed is s with owner's whole hand swapped for as many cards from the
// bottom of the draw pile, which no step draws while the pile holds more
// than a step can take above them; false when they are the same cards.
func perturbed(s state, owner int) (state, bool) {
	hand := s.hands[owner]
	below := len(s.pile) - len(hand)
	if len(hand) == 0 || below < failedChallenge || sameCards(hand, s.pile[below:]) {
		return s, false
	}
	s.hands = slices.Clone(s.hands)
	s.hands[owner] = slices.Clone(s.pile[below:])
	s.pile = append(slices.Clone(s.pile[:below]), hand...)
	return s, true
}

// redealt is the game New would deal from o's deck with owner's dealt cards
// traded for others, and the deal's events, with the seats dealt either. The
// others are the deck's bottom cards, unless a Wild Draw Four turned up made
// the deal reshuffle the rest of the deck, which must then stay as it was:
// the next seat's dealt cards. False when the two hold the same cards.
func redealt(o opened, owner int) (opened, []game.Seat, bool) {
	s := o.state
	n := len(s.order)
	dealer := slices.Index(s.order, o.deal[0].(StartedEvent).Dealer)
	dealtTo := func(i int) []int {
		var at []int
		for k := range handSize * n {
			if (dealer+1+k)%n == i {
				at = append(at, k)
			}
		}
		return at
	}
	deck := cards.Shuffled(orderedDeck(), s.subseed, 0)
	mine := dealtTo(owner)
	others := make([]int, handSize)
	for k := range others {
		others[k] = len(deck) - handSize + k
	}
	seen := []game.Seat{s.order[owner]}
	if s.shuffles > 0 {
		next := (owner + 1) % n
		others, seen = dealtTo(next), append(seen, s.order[next])
	}
	pick := func(at []int) []Card { return xslices.Map(at, func(k int) Card { return deck[k] }) }
	if sameCards(pick(mine), pick(others)) {
		return o, nil, false
	}
	for k := range mine {
		deck[mine[k]], deck[others[k]] = deck[others[k]], deck[mine[k]]
	}
	changed, events := dealFrom(s.order, dealer, s.subseed, deck)
	return opened{state: changed, deal: events}, seen, true
}

// otherHand changes owner's hidden hand, and returns the seats that may see it.
func otherHand(st game.State[Action, View], owner game.Seat) (game.State[Action, View], []game.Seat) {
	if o, fresh := st.(opened); fresh {
		changed, seen, ok := redealt(o, slices.Index(o.order, owner))
		if !ok {
			return st, nil
		}
		return changed, seen
	}
	s := st.(state)
	changed, ok := perturbed(s, slices.Index(s.order, owner))
	if !ok {
		return st, nil
	}
	return changed, []game.Seat{owner}
}

// challengeTellsEverySeat is a challenge of the Wild Draw Four an owner
// played: its hand decides the verdict, which every seat is told, and who
// draws the four or six cards.
func challengeTellsEverySeat(st game.State[Action, View], step gametest.Step[Action], owners []game.Seat) []game.Seat {
	s, ok := st.(state)
	if !ok || step.Action.Kind != ActionChallenge || !slices.Contains(owners, s.order[s.offender]) {
		return nil
	}
	return s.order
}

// TestAHandReachesNoOtherSeat changes one seat's hidden hand and requires
// what every other seat is sent to stay byte for byte the same. The one
// thing a hand may change is the verdict of a challenge against it. Three
// players from seed 1 reach a challenge whose verdict a changed hand flips.
func TestAHandReachesNoOtherSeat(t *testing.T) {
	type table struct {
		players int
		seed    game.Seed
	}
	tables := []table{{3, 1}, {4, 134}}
	for n := playerRange.Min; n <= playerRange.Max; n++ {
		tables = append(tables, table{n, game.Seed(20 + n)})
	}
	for _, tt := range tables {
		steps, _ := playToFinish(t, tt.players, tt.seed)
		users := gametest.Users(tt.players)
		script := gametest.Script[Opts, Action]{Challenger: users[0], Invitees: users[1:], Seed: tt.seed, Steps: steps}
		gametest.NonInterference(t, Rules{}, script, gametest.Hidden[Action, View]{Perturb: otherHand, Probes: probes, Reveals: challengeTellsEverySeat})
	}
}

func viewsBut(t *testing.T, s state, except ...int) []string {
	t.Helper()
	out := []string{}
	for i, at := range s.order {
		if !slices.Contains(except, i) {
			out = append(out, gametest.ViewJSON(t, s.View(at)))
		}
	}
	return out
}

// TestADrawnCardReachesOnlyItsDrawer turns the draw pile upside down, so a
// step draws other cards, and requires no event of a step, nor the view of any seat that drew
// nothing in it, to change.
func TestADrawnCardReachesOnlyItsDrawer(t *testing.T) {
	checked := 0
	for n := playerRange.Min; n <= playerRange.Max; n++ {
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
	if diff := cmp.Diff(events, changedEvents); diff != "" {
		t.Fatalf("%s: events depend on it:\n%s", what, diff)
	}
	if diff := cmp.Diff(viewsBut(t, next.(state), except...), viewsBut(t, changedNext.(state), except...)); diff != "" {
		t.Fatalf("%s: views after the step depend on it:\n%s", what, diff)
	}
}
