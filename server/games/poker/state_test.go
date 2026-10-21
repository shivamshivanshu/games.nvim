package poker

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
)

func full(n int) []Chips {
	stacks := make([]Chips, n)
	for i := range stacks {
		stacks[i] = startingStack
	}
	return stacks
}

// limpPreflop calls round and checks the big blind's option.
func limpPreflop(t *testing.T, s state) state {
	t.Helper()
	for s.hand.dealt == 0 && !s.finished {
		i := s.hand.toAct
		s, _ = play(t, s, i, "call")
	}
	return s
}

func kinds(events []game.Event) []game.EventKind {
	out := make([]game.EventKind, len(events))
	for i, e := range events {
		out[i] = e.EventKind()
	}
	return out
}

func TestBlindsAndFirstToAct(t *testing.T) {
	tests := []struct {
		name                     string
		players, button          int
		small, big               int
		firstPreflop, firstLater int
	}{
		{"heads-up: the button posts the small blind and acts first preflop, last after", 2, 0, 0, 1, 0, 1},
		{"three-handed", 3, 0, 1, 2, 0, 1},
		{"four-handed", 4, 3, 0, 1, 2, 0},
		{"six-handed", 6, 2, 3, 4, 5, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := table(t, 1, tt.button, full(tt.players)...)
			v := s.View(seatName(0))
			if v.Seats[tt.small].Bet != firstSmallBlind || v.Seats[tt.big].Bet != firstBigBlind {
				t.Errorf("bets %+v, want small blind on p%d and big on p%d", v.Seats, tt.small+1, tt.big+1)
			}
			if s.hand.toAct != tt.firstPreflop {
				t.Errorf("first preflop p%d, want p%d", s.hand.toAct+1, tt.firstPreflop+1)
			}
			if s = limpPreflop(t, s); s.hand.toAct != tt.firstLater {
				t.Errorf("first on the flop p%d, want p%d", s.hand.toAct+1, tt.firstLater+1)
			}
		})
	}
}

func TestBigBlindHasTheOptionAfterLimps(t *testing.T) {
	s := table(t, 1, 0, full(3)...)
	s, _ = playAll(t, s, "p1:call, p2:call")
	if s.hand.toAct != 2 || !slices.Contains(s.View(seatName(2)).Legal, MoveRaise) {
		t.Fatalf("big blind p3 should act with a raise option; to act p%d", s.hand.toAct+1)
	}
	raised, _ := play(t, s, 2, "raise 60")
	if raised.hand.dealt != 0 || raised.hand.toAct != 0 {
		t.Errorf("a raise by the big blind must keep preflop going to p1")
	}
	checked, events := play(t, s, 2, "check")
	if checked.hand.dealt != 3 || len(eventsOf[BoardEvent](events)) != 1 {
		t.Errorf("the big blind's check must deal the flop")
	}
}

func TestButtonMovesToTheNextSeatWithChips(t *testing.T) {
	tests := []struct {
		name       string
		stacks     []Chips
		button     int
		fold       string
		wantButton int
		wantSmall  int
		wantBig    int
	}{
		{"heads-up after a bust swaps blinds", []Chips{1000, 0, 1000}, 0, "p1:fold", 2, 2, 0},
		{"skips a busted seat", []Chips{1000, 1000, 0, 1000}, 1, "p2:fold, p4:fold", 3, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := playAll(t, table(t, 1, tt.button, tt.stacks...), tt.fold)
			if s.hand.number != 2 {
				t.Fatalf("still on hand %d", s.hand.number)
			}
			bets := s.View(seatName(0)).Seats
			if s.hand.button != tt.wantButton || bets[tt.wantSmall].Bet != firstSmallBlind || bets[tt.wantBig].Bet != firstBigBlind {
				t.Errorf("button p%d, seats %+v; want button p%d, small p%d, big p%d", s.hand.button+1, bets, tt.wantButton+1, tt.wantSmall+1, tt.wantBig+1)
			}
		})
	}
}

func TestShortStackPostsAllInAndOthersStillCallTheFullBlind(t *testing.T) {
	s := table(t, 1, 0, 1000, 1000, 15)
	v := s.View(seatName(0))
	if bb := v.Seats[2]; bb.Bet != 15 || !bb.AllIn {
		t.Errorf("short big blind %+v, want all-in for 15", bb)
	}
	if v.ToCall != firstBigBlind {
		t.Errorf("p1 to call %d, want the full big blind %d", v.ToCall, firstBigBlind)
	}
}

func TestHeadsUpShortBigBlindIsRunOutWithoutAMove(t *testing.T) {
	s := state{n: 2, subseed: 1}
	s.hand.chips.stack = [maxPlayers]Chips{1000, 5}
	s, dealt := s.dealHand("", 1, 0)
	_, events := s.advance("", ReasonAllChips)
	events = append(dealt, events...)
	want := []game.EventKind{EventHandStarted, EventPosted, EventPosted, EventBoard, EventBoard, EventBoard, EventShown, EventReturned, EventWon}
	if diff := cmp.Diff(want, kinds(events[:len(want)])); diff != "" {
		t.Fatalf("events (-want +got):\n%s", diff)
	}
	if got := events[len(want)-2].(ReturnedEvent); got.Seat != "p1" || got.Amount != 5 {
		t.Errorf("returned %+v, want the small blind's 5 over the all-in big blind", got)
	}
	if got := events[len(want)-1].(WonEvent); got.Amount != 10 {
		t.Errorf("won %+v, want the 10-chip pot", got)
	}
}

func TestBlindsDoubleEveryTenHands(t *testing.T) {
	tests := []struct {
		hand int
		want Blinds
	}{
		{1, Blinds{10, 20}},
		{10, Blinds{10, 20}},
		{11, Blinds{20, 40}},
		{21, Blinds{40, 80}},
		{31, Blinds{80, 160}},
	}
	for _, tt := range tests {
		if got := blindsFor(tt.hand); got != tt.want {
			t.Errorf("hand %d: blinds %+v, want %+v", tt.hand, got, tt.want)
		}
	}
	if got := table(t, 11, 0, full(2)...).View(seatName(0)).Blinds; got != (Blinds{20, 40}) {
		t.Errorf("hand 11 posts %+v", got)
	}
}

func TestAllInIsShownAndRunOutInTheSameApply(t *testing.T) {
	s := withCards(t, table(t, 1, 0, full(2)...), map[int]string{0: "As Ah", 1: "Kd Kc"}, "2c 7d 9h Js 3c")
	s, events := playAll(t, s, "p1:allin, p2:call")
	want := []game.EventKind{EventActed, EventBoard, EventBoard, EventBoard, EventShown, EventWon, EventBusted}
	if diff := cmp.Diff(want, kinds(events[1:])); diff != "" {
		t.Fatalf("events (-want +got):\n%s", diff)
	}
	shown := eventsOf[ShownEvent](events)[0].Hands
	wantShown := []ShownHand{{"p2", parseCards(t, "Kd Kc"), Pair}, {"p1", parseCards(t, "As Ah"), Pair}}
	if diff := cmp.Diff(wantShown, shown); diff != "" {
		t.Errorf("shown (-want +got):\n%s", diff)
	}
	result, finished := s.Outcome()
	if !finished || !reflect.DeepEqual(result, game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{"p1"}, Reason: ReasonAllChips}) {
		t.Errorf("result %+v finished %t", result, finished)
	}
	if diff := cmp.Diff(map[game.Seat]int64{"p1": 1000, "p2": -1000}, s.Scores()); diff != "" {
		t.Errorf("scores (-want +got):\n%s", diff)
	}
}

func TestFoldWinRevealsNothing(t *testing.T) {
	s, events := play(t, table(t, 1, 0, full(2)...), 0, "fold")
	last := s.View("p1").LastHand
	if len(eventsOf[ShownEvent](events)) != 0 || len(last.Shown) != 0 {
		t.Errorf("a fold win showed hands: events %v, last %+v", events, last)
	}
	if diff := cmp.Diff([]Award{{Seat: "p2", Amount: 10}}, last.Awards); diff != "" {
		t.Errorf("awards (-want +got):\n%s", diff)
	}
	if returned := eventsOf[ReturnedEvent](events); len(returned) != 1 || returned[0].Amount != 20 {
		t.Errorf("returned %+v, want the big blind's whole 20: a folded blind matches nothing", returned)
	}
}

func TestSidePotsGoToTheBestEligibleHandFromTheLastPotDown(t *testing.T) {
	tests := []struct {
		name       string
		holes      map[int]string
		wantWon    []WonEvent
		wantStacks []Chips
	}{
		{
			"shortest stack best",
			map[int]string{0: "As Ad", 1: "Ks Kd", 2: "Qs Qd"},
			[]WonEvent{{By: "p2", Seat: "p2", Amount: 400, Pot: 1}, {By: "p2", Seat: "p1", Amount: 300}},
			[]Chips{300, 400, 700},
		},
		{
			"biggest stack best",
			map[int]string{0: "Qs Qd", 1: "Ks Kd", 2: "As Ad"},
			[]WonEvent{{By: "p2", Seat: "p3", Amount: 400, Pot: 1}, {By: "p2", Seat: "p3", Amount: 300}},
			[]Chips{0, 0, 1400},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := withCards(t, table(t, 1, 2, 100, 300, 1000), tt.holes, "2c 7h 9h Js 3c")
			s, events := playAll(t, s, "p3:allin, p1:allin, p2:allin")
			if diff := cmp.Diff(tt.wantWon, eventsOf[WonEvent](events)); diff != "" {
				t.Errorf("won (-want +got):\n%s", diff)
			}
			if returned := eventsOf[ReturnedEvent](events); len(returned) != 0 {
				t.Errorf("returned %+v: p3's shove commits only the 300 an opponent can match", returned)
			}
			var got []Chips
			for i := range 3 {
				got = append(got, s.hand.chips.stack[i]+s.hand.chips.committed[i])
			}
			if diff := cmp.Diff(tt.wantStacks, got); diff != "" {
				t.Errorf("chips after the hand (-want +got):\n%s", diff)
			}
		})
	}
}

// Three all-ins of different sizes and a caller make a main pot and two side
// pots; the two tied hands split both side pots.
func TestThreeAllInsMakeSidePotsThatTiedHandsSplit(t *testing.T) {
	s := withCards(t, table(t, 1, 0, 1000, 200, 400, 100),
		map[int]string{0: "Qs 3d", 1: "8c 4s", 2: "Qd 3h", 3: "As Ad"}, "2c 7d 9h Jc Ks")
	s, events := playAll(t, s, "p4:allin, p1:call, p2:allin, p3:allin, p1:call")
	want := []WonEvent{
		{By: "p1", Seat: "p3", Amount: 200, Pot: 2}, {By: "p1", Seat: "p1", Amount: 200, Pot: 2},
		{By: "p1", Seat: "p3", Amount: 150, Pot: 1}, {By: "p1", Seat: "p1", Amount: 150, Pot: 1},
		{By: "p1", Seat: "p4", Amount: 400},
	}
	if diff := cmp.Diff(want, eventsOf[WonEvent](events)); diff != "" {
		t.Errorf("won (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]BustedEvent{{By: "p1", Seat: "p2"}}, eventsOf[BustedEvent](events)); diff != "" {
		t.Errorf("busted (-want +got):\n%s", diff)
	}
	if got := s.KnockedOut(); !slices.Equal(got, []game.Seat{"p2"}) {
		t.Errorf("knocked out %v, want the busted p2, so the core takes its controls", got)
	}
	chips := s.hand.chips
	if got := []Chips{chips.stack[0] + chips.committed[0], chips.stack[2] + chips.committed[2], chips.stack[3] + chips.committed[3]}; !slices.Equal(got, []Chips{950, 350, 400}) {
		t.Errorf("p1, p3, p4 hold %v, want [950 350 400]", got)
	}
}

func TestSplitPotOddChipsGoClockwiseFromTheLeftOfTheButton(t *testing.T) {
	s := withCards(t, table(t, 1, 0, full(4)...), map[int]string{0: "2c 3d", 1: "4c 5d", 2: "6c 7d", 3: "8c 9d"}, "As Ks Qs Js Ts")
	s, events := playAll(t, s, "p4:call, p1:call, p2:fold, p3:check"+
		", p3:check, p4:check, p1:check"+
		", p3:check, p4:check, p1:check"+
		", p3:check, p4:check, p1:check")
	want := []WonEvent{{By: "p1", Seat: "p3", Amount: 24}, {By: "p1", Seat: "p4", Amount: 23}, {By: "p1", Seat: "p1", Amount: 23}}
	if diff := cmp.Diff(want, eventsOf[WonEvent](events)); diff != "" {
		t.Errorf("won (-want +got):\n%s", diff)
	}
	if s.hand.number != 2 {
		t.Errorf("hand %d, want the next hand dealt", s.hand.number)
	}
}

func TestEliminateFoldsTheSeatAndLeavesEveryChipItPutInInThePot(t *testing.T) {
	s := table(t, 1, 0, full(3)...)
	next, events := s.Eliminate("p3", game.ReasonResign)
	s = next.(state)
	if diff := cmp.Diff([]game.Event{LeftEvent{By: "p3"}}, events); diff != "" {
		t.Errorf("events (-want +got):\n%s", diff)
	}
	if v := s.View("p1"); !v.Seats[2].Out || !v.Seats[2].Folded || v.Seats[2].Stack != 0 || s.hand.toAct != 0 {
		t.Fatalf("after p3 left: seats %+v, to act p%d", v.Seats, s.hand.toAct+1)
	}
	s, events = play(t, s, 0, "fold")
	if diff := cmp.Diff([]WonEvent{{By: "p1", Seat: "p2", Amount: 20}}, eventsOf[WonEvent](events)); diff != "" {
		t.Errorf("p2 must win all of p3's 20 (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]ReturnedEvent{{By: "p1", Seat: "p2", Amount: 10}}, eventsOf[ReturnedEvent](events)); diff != "" {
		t.Errorf("only p2 gets its own chips back, never the seat that left (-want +got):\n%s", diff)
	}
	if s.hand.number != 2 || s.hand.dealtIn[2] {
		t.Errorf("hand 2 must be dealt without p3")
	}
}

func TestEliminateAnAllInSeatLeavesItsUncalledRaiseInThePot(t *testing.T) {
	s, _ := playAll(t, table(t, 1, 0, 1000, 300, 1000), "p1:allin, p2:allin")
	next, _ := s.Eliminate("p1", game.ReasonResign)
	_, events := play(t, next.(state), 2, "fold")
	if diff := cmp.Diff([]WonEvent{{By: "p3", Seat: "p2", Amount: 1020}}, eventsOf[WonEvent](events)); diff != "" {
		t.Errorf("p2 must win p1's whole 1000 and p3's 20 (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]ReturnedEvent{{By: "p3", Seat: "p2", Amount: 300}}, eventsOf[ReturnedEvent](events)); diff != "" {
		t.Errorf("only p2's own 300 comes back; p1's uncalled raise stays in the pot (-want +got):\n%s", diff)
	}
}

// A shoves 1000 and leaves, B is all-in for 300 and C has 1000: C's call and
// C's allin both commit 300, and A's dead 700 goes to whichever of B and C
// wins, never to C uncontested.
func TestALeaversDeadChipsAreContestedByTheSeatsStillInTheHand(t *testing.T) {
	tests := []struct {
		name  string
		holes map[int]string
		best  game.Seat
	}{
		{"the all-in seat has the best hand", map[int]string{0: "2s 3d", 1: "As Ad", 2: "Ks Kd"}, "p2"},
		{"the seat with chips behind has the best hand", map[int]string{0: "2s 3d", 1: "Ks Kd", 2: "As Ad"}, "p3"},
	}
	for _, tt := range tests {
		for _, move := range []string{"call", "allin"} {
			t.Run(tt.name+" after "+move, func(t *testing.T) {
				s := withCards(t, table(t, 1, 0, 1000, 300, 1000), tt.holes, "2c 7h 9h Js 4c")
				s, _ = playAll(t, s, "p1:allin, p2:allin")
				left, _ := s.Eliminate("p1", game.ReasonResign)
				s, events := play(t, left.(state), 2, move)
				if diff := cmp.Diff([]WonEvent{{By: "p3", Seat: tt.best, Amount: 1600}}, eventsOf[WonEvent](events)); diff != "" {
					t.Errorf("one pot of 300 each from p2 and p3 and p1's whole 1000 (-want +got):\n%s", diff)
				}
				if behind := s.hand.chips.stack[2] + s.hand.chips.committed[2]; tt.best == "p2" && behind != 700 {
					t.Errorf("p3 holds %d, want the 700 no one could match left behind", behind)
				}
			})
		}
	}
}

// A bets 500, B calls and leaves, and C is all-in for 300. B's dead 500
// matches nothing: A gets back the 200 above C, and the rest is one pot that
// A and C contest.
func TestADeadCallMatchesNothing(t *testing.T) {
	tests := []struct {
		name  string
		holes map[int]string
		best  game.Seat
	}{
		{"the all-in seat has the best hand", map[int]string{0: "Ks Kd", 1: "2s 3d", 2: "As Ad"}, "p3"},
		{"the bettor has the best hand", map[int]string{0: "As Ad", 1: "2s 3d", 2: "Ks Kd"}, "p1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := withCards(t, table(t, 1, 0, 1000, 1000, 300), tt.holes, "2c 7h 9h Js 4c")
			s, _ = playAll(t, s, "p1:raise 500, p2:call, p3:allin")
			_, events := s.Eliminate("p2", game.ReasonResign)
			if diff := cmp.Diff([]ReturnedEvent{{By: "p2", Seat: "p1", Amount: 200}}, eventsOf[ReturnedEvent](events)); diff != "" {
				t.Errorf("returned (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff([]WonEvent{{By: "p2", Seat: tt.best, Amount: 1100}}, eventsOf[WonEvent](events)); diff != "" {
				t.Errorf("one pot of 300 each from p1 and p3 and p2's whole 500 (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEliminateTheSeatToActPassesTheTurn(t *testing.T) {
	next, _ := table(t, 1, 0, full(3)...).Eliminate("p1", game.ReasonResign)
	if got := next.ToAct(); !reflect.DeepEqual(got, []game.Seat{"p2"}) {
		t.Errorf("to act %v, want p2", got)
	}
}

func TestEliminateHeadsUpEndsTheGameWithTheReason(t *testing.T) {
	next, _ := table(t, 1, 0, full(2)...).Eliminate("p1", game.ReasonResign)
	result, finished := next.Outcome()
	if !finished || !reflect.DeepEqual(result, game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{"p2"}, Reason: game.ReasonResign}) {
		t.Errorf("result %+v finished %t", result, finished)
	}
}

func TestScoresAreZeroSumAfterASeatLeavesWithChips(t *testing.T) {
	next, _ := table(t, 1, 0, full(3)...).Eliminate("p3", game.ReasonResign)
	s := next.(state)
	s, _ = play(t, s, 0, "fold")
	s = withCards(t, s, map[int]string{0: "Ks Kd", 1: "As Ad"}, "2c 7h 9h Js 3c")
	s, _ = playAll(t, s, "p2:allin, p1:call")
	if _, finished := s.Outcome(); !finished {
		t.Fatal("game should be over")
	}
	if diff := cmp.Diff(map[game.Seat]int64{"p1": -1000, "p2": 2000, "p3": -1000}, s.Scores()); diff != "" {
		t.Errorf("scores (-want +got):\n%s", diff)
	}
}

func TestApplyRejectsIllegalActions(t *testing.T) {
	s := table(t, 1, 0, full(2)...)
	over, _ := s.Eliminate("p2", game.ReasonResign)
	tests := []struct {
		name   string
		state  game.State[Action, View]
		seat   game.Seat
		action Action
		want   string
	}{
		{"unknown kind", s, "p1", Action{Kind: "move", Text: "call"}, `unknown action "move"`},
		{"empty", s, "p1", Action{Kind: ActionAct, Text: "  "}, "empty move"},
		{"unknown move", s, "p1", Action{Kind: ActionAct, Text: "shove"}, `unknown move "shove"`},
		{"amount on fold", s, "p1", Action{Kind: ActionAct, Text: "fold 20"}, "fold takes no amount"},
		{"raise without an amount", s, "p1", Action{Kind: ActionAct, Text: "raise"}, "raise needs one amount"},
		{"amount not a number", s, "p1", Action{Kind: ActionAct, Text: "raise lots"}, `raise amount "lots"`},
		{"negative amount", s, "p1", Action{Kind: ActionAct, Text: "raise -40"}, `raise amount "-40"`},
		{"bet preflop", s, "p1", Action{Kind: ActionAct, Text: "bet 100"}, "a bet is already open"},
		{"check facing the big blind", s, "p1", Action{Kind: ActionAct, Text: "check"}, "cannot check: 10 to call"},
		{"game over", over, "p1", Action{Kind: ActionAct, Text: "call"}, "game is over"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tt.state.Apply(tt.seat, tt.action, nil)
			wantIllegal(t, err, tt.want)
		})
	}
	for _, seat := range []game.Seat{"p2", "p3", "x"} {
		if _, _, err := s.Apply(seat, Action{Kind: ActionAct, Text: "call"}, nil); !errors.Is(err, game.ErrNotYourTurn) {
			t.Errorf("%s: error %v, want not your turn", seat, err)
		}
	}
}

func TestMovesAreCaseAndSpaceInsensitive(t *testing.T) {
	s, events := play(t, table(t, 1, 0, full(2)...), 0, "  RAISE   60 ")
	if diff := cmp.Diff(ActedEvent{By: "p1", Move: MoveRaise, Amount: 60}, events[0]); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
	if s.hand.toAct != 1 {
		t.Errorf("to act p%d", s.hand.toAct+1)
	}
}

func TestApplyLeavesTheEarlierStateUnchanged(t *testing.T) {
	s, _ := play(t, table(t, 1, 0, full(3)...), 0, "fold")
	before := s
	lastBefore, err := json.Marshal(s.ended.summary())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Apply(seatName(s.hand.toAct), Action{Kind: ActionAct, Text: "allin"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _ = s.Eliminate("p2", game.ReasonResign); !reflect.DeepEqual(before, s) {
		t.Error("Apply or Eliminate changed its receiver")
	}
	if lastAfter, _ := json.Marshal(s.ended.summary()); string(lastAfter) != string(lastBefore) {
		t.Errorf("last hand changed: %s -> %s", lastBefore, lastAfter)
	}
}

func TestNewSeatsTwoToSixPlayers(t *testing.T) {
	for _, n := range []int{1, 7} {
		users := make([]game.UserID, n)
		for i := range users {
			users[i] = game.UserID(rune('a' + i))
		}
		_, _, err := Rules{}.New(Opts{}, users[0], users[1:], game.StepRNG(1, 0))
		if _, ok := errors.AsType[game.BadRequestError](err); !ok {
			t.Errorf("%d players: error %v, want bad request", n, err)
		}
	}
	st, seats, err := Rules{}.New(Opts{}, "alice", []game.UserID{"bob", "carol"}, game.StepRNG(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[game.UserID]game.Seat{"alice": "p1", "bob": "p2", "carol": "p3"}, seats); diff != "" {
		t.Errorf("seats (-want +got):\n%s", diff)
	}
	var chips Chips
	for _, seat := range st.View("p1").Seats {
		chips += seat.Stack + seat.Bet
	}
	if chips != 3*startingStack {
		t.Errorf("table holds %d chips, want %d", chips, 3*startingStack)
	}
}

func TestMutatingAViewOrEventLeavesTheStateUnchanged(t *testing.T) {
	s := withCards(t, table(t, 1, 0, full(3)...), map[int]string{0: "As Ah", 1: "Kd Kc", 2: "Qd Qc"}, "2c 7d 9h Js 3c")
	s, events := playAll(t, s, "p1:allin, p2:allin, p3:fold")
	before, err := json.Marshal(s.View("p1"))
	if err != nil {
		t.Fatal(err)
	}
	v := s.View("p1")
	v.Board = append(v.Board[:0], cards.Card{Rank: cards.Ace, Suit: cards.Spades})
	v.MyCards[0] = cards.Card{Rank: cards.Two, Suit: cards.Clubs}
	v.LastHand.Board[0] = cards.Card{Rank: cards.Two, Suit: cards.Clubs}
	v.LastHand.Shown[0].Cards[0] = cards.Card{Rank: cards.Two, Suit: cards.Clubs}
	v.LastHand.Awards[0].Amount = 1
	shown := eventsOf[ShownEvent](events)[0]
	shown.Hands[0].Cards[0] = cards.Card{Rank: cards.Two, Suit: cards.Clubs}
	eventsOf[BoardEvent](events)[0].Cards[0] = cards.Card{Rank: cards.Two, Suit: cards.Clubs}
	if after, _ := json.Marshal(s.View("p1")); string(after) != string(before) {
		t.Errorf("view changed through a returned view or event:\n%s\n%s", before, after)
	}
}

func TestViewPotsAndBettingNumbers(t *testing.T) {
	type numbers struct {
		Legal                          []MoveKind
		ToCall, MinRaiseTo, MaxRaiseTo Chips
	}
	none := numbers{Legal: []MoveKind{}}
	tests := []struct {
		name        string
		stacks      []Chips
		moves       string
		viewer      game.Seat
		wantPots    []PotView
		wantNumbers numbers
	}{
		{
			"settled pots leave out the street's bets",
			full(3), "p1:call, p2:call, p3:check, p2:bet 40", "p3",
			[]PotView{{60, []game.Seat{"p1", "p2", "p3"}}},
			numbers{[]MoveKind{MoveFold, MoveCall, MoveRaise, MoveAllIn}, 40, 80, 980},
		},
		{
			"a folded viewer is not eligible and gets no numbers",
			full(3), "p1:fold, p2:call, p3:check", "p1",
			[]PotView{{40, []game.Seat{"p2", "p3"}}},
			none,
		},
		{
			"a viewer with chips who is not to act gets no numbers",
			full(3), "p1:call", "p1",
			[]PotView{},
			none,
		},
		{
			"a short stack facing a raise gets no raise range",
			[]Chips{1000, 1000, 60}, "p1:raise 100, p2:fold", "p3",
			[]PotView{},
			numbers{[]MoveKind{MoveFold, MoveCall, MoveAllIn}, 40, 0, 0},
		},
		{
			"an earlier all-in caps a side pot, and the all-in viewer gets no numbers",
			[]Chips{1000, 50, 1000}, "p1:raise 100, p2:allin, p3:call", "p2",
			[]PotView{{150, []game.Seat{"p1", "p2", "p3"}}, {100, []game.Seat{"p1", "p3"}}},
			none,
		},
		{
			"an out viewer gets no numbers",
			[]Chips{1000, 0, 1000}, "", "p2",
			[]PotView{},
			none,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := table(t, 1, 0, tt.stacks...)
			if tt.moves != "" {
				s, _ = playAll(t, s, tt.moves)
			}
			v := s.View(tt.viewer)
			if diff := cmp.Diff(tt.wantPots, v.Pots); diff != "" {
				t.Errorf("pots (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantNumbers, numbers{v.Legal, v.ToCall, v.MinRaiseTo, v.MaxRaiseTo}); diff != "" {
				t.Errorf("legal, to_call, min_raise_to, max_raise_to (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConcludeLeavesNoSeatToActAndOffersNoMoves(t *testing.T) {
	s := table(t, 1, 0, full(3)...)
	aborted := game.Result{Kind: game.ResultAborted}
	concluded := s.Conclude(aborted)
	if got := concluded.ToAct(); len(got) != 0 {
		t.Errorf("to act after conclude: %v", got)
	}
	if result, done := concluded.Outcome(); !done || result.Kind != game.ResultAborted {
		t.Errorf("outcome %+v, %t; want the imposed abort", result, done)
	}
	if v := concluded.View(seatName(s.hand.toAct)); len(v.Legal) != 0 || v.ToCall != 0 {
		t.Errorf("concluded view offers %v, %d to call", v.Legal, v.ToCall)
	}
}

func TestScoresAfterAnImposedEndingAreZeroForEverySeat(t *testing.T) {
	concluded := table(t, 1, 0, full(3)...).Conclude(game.Result{Kind: game.ResultAborted}).(state)
	if diff := cmp.Diff(map[game.Seat]int64{"p1": 0, "p2": 0, "p3": 0}, concluded.Scores()); diff != "" {
		t.Errorf("scores (-want +got):\n%s", diff)
	}
}
