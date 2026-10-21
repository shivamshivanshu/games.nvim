package hearts

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
)

func TestNewTakesExactlyFourAndDealsThirteenEach(t *testing.T) {
	for n := 1; n <= 5; n++ {
		all := append(users(), "erin")[:n]
		_, _, err := Rules{}.New(Opts{}, all[0], all[1:], game.StepRNG(1, 0))
		if (n == players) != (err == nil) {
			t.Fatalf("%d players: err %v", n, err)
		}
	}
	s := newGame(t, 1)
	seen := map[cards.Card]bool{}
	for _, h := range s.hands {
		if len(h) != handSize {
			t.Fatalf("a hand of %d", len(h))
		}
		for _, c := range h {
			seen[c] = true
		}
	}
	if len(seen) != cards.FrenchSize || s.phase != PhasePass || len(s.ToAct()) != players {
		t.Errorf("%d cards, phase %s, to act %v", len(seen), s.phase, s.ToAct())
	}
}

func TestPassesGoLeftRightAcrossThenHold(t *testing.T) {
	tests := []struct {
		handNo int
		pass   PassDirection
		to     int
	}{
		{1, PassLeft, 1}, {2, PassRight, 3}, {3, PassAcross, 2}, {4, PassHold, 0}, {5, PassLeft, 1},
	}
	for _, tt := range tests {
		s := newGame(t, 1).deal(tt.handNo)
		if passOf(tt.handNo) != tt.pass {
			t.Errorf("hand %d passes %s, want %s", tt.handNo, passOf(tt.handNo), tt.pass)
		}
		if tt.pass == PassHold {
			if s.phase != PhasePlay || !slices.Contains(s.hands[s.turn], twoOfClubs) {
				t.Errorf("held hand: phase %s, %s to lead without the two of clubs", s.phase, s.order[s.turn])
			}
			continue
		}
		if got := s.receiver(0); got != tt.to {
			t.Errorf("hand %d: p1 passes to seat index %d, want %d", tt.handNo, got, tt.to)
		}
	}
}

func TestThePassChangesHandsOnceEverySeatHasChosen(t *testing.T) {
	s := newGame(t, 2)
	chosen := make([][]cards.Card, players)
	for i, at := range s.order {
		chosen[i] = highest(s.hands[i])
		var events []game.Event
		s, events = apply(t, s, at, passing(strings.ToLower(texts(chosen[i]))))
		if events[0] != (PassChosenEvent{By: at, Seat: at}) {
			t.Fatalf("events %v", events)
		}
		if i < players-1 && (s.phase != PhasePass || slices.Contains(s.ToAct(), at)) {
			t.Fatalf("after %s chose: phase %s, to act %v", at, s.phase, s.ToAct())
		}
		if i < players-1 {
			if v := s.View(at); !slices.Equal(v.Passed, chosen[i]) || v.MustPass != 0 || !v.Seats[i].PassChosen {
				t.Errorf("%s's view after choosing: %+v", at, v)
			}
		}
	}
	if s.phase != PhasePlay || !slices.Contains(s.hands[s.turn], twoOfClubs) {
		t.Fatalf("after every pass: phase %s, %s leads", s.phase, s.order[s.turn])
	}
	for i, at := range s.order {
		v := s.View(at)
		left := s.order[(i+1)%players]
		right := (i + players - 1) % players
		if !slices.Equal(v.Received, chosen[right]) || !slices.Equal(v.Passed, chosen[i]) {
			t.Errorf("%s received %v from the right, passed %v", at, v.Received, v.Passed)
		}
		for _, c := range chosen[i] {
			if slices.Contains(v.MyHand, c) || !slices.Contains(s.View(left).MyHand, c) {
				t.Errorf("%s's passed %s did not reach %s", at, c, left)
			}
		}
	}
}

func TestPassRejects(t *testing.T) {
	s := newGame(t, 3)
	mine := s.hands[0]
	notMine := s.hands[1][0]
	tests := []struct {
		name string
		by   game.Seat
		a    Action
		want string
	}{
		{"a card not held", seat(1), passing(texts([]cards.Card{mine[0], mine[1], notMine})), "you hold no " + notMine.String()},
		{"a card twice", seat(1), passing(texts([]cards.Card{mine[0], mine[0], mine[1]})), "names a card twice"},
		{"two cards", seat(1), passing(texts(mine[:2])), "is not three cards"},
		{"playing before passing", seat(1), playing(mine[0].String()), "you cannot play now: choose three cards to pass left"},
		{"an unknown kind", seat(1), Action{Kind: "bid"}, `unknown action "bid"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := s.Apply(tt.by, tt.a, nil); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err %v, want %q", err, tt.want)
			}
		})
	}
	chose, _ := apply(t, s, seat(1), passing(texts(mine[:3])))
	if _, _, err := chose.Apply(seat(1), passing(texts(mine[3:6])), nil); err != game.ErrNotYourTurn {
		t.Errorf("choosing twice: %v", err)
	}
}

func TestPlayRules(t *testing.T) {
	tests := []struct {
		name    string
		trickNo int
		broken  bool
		lead    string
		hands   []string
		by      int
		play    string
		want    string
	}{
		{"the first trick opens with the two of clubs", 1, false, "", []string{"2c 3c", "4c", "5c", "6c"}, 1, "3c", "the first trick is led with the 2c"},
		{"a heart cannot lead before hearts break", 2, false, "", []string{"2h 3c", "4c", "5c", "6c"}, 1, "2h", "hearts are not broken"},
		{"a seat must follow the suit led", 2, false, "Kd", []string{"Kd", "4c 2d", "5c", "6c"}, 2, "4c", "you must follow diamonds"},
		{"no points on the first trick when void", 1, false, "2c", []string{"2c", "Qs 4d", "5c", "6c"}, 2, "Qs", "no points on the first trick"},
		{"a card not held", 2, false, "", []string{"2h 3c", "4c", "5c", "6c"}, 1, "As", "you hold no As"},
		{"text that is no card", 2, false, "", []string{"2h 3c", "4c", "5c", "6c"}, 1, "10h", `"10h" is no card`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tricks(t, tt.trickNo, tt.broken, tt.hands...)
			if tt.lead != "" {
				s, _ = apply(t, s, seat(1), playing(tt.lead))
			}
			if _, _, err := s.Apply(seat(tt.by), playing(tt.play), nil); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err %v, want %q", err, tt.want)
			}
			if _, _, err := s.Apply(seat(tt.by%players+1), playing(tt.play), nil); err != game.ErrNotYourTurn {
				t.Errorf("out of turn: %v", err)
			}
		})
	}
}

func TestLegalCards(t *testing.T) {
	tests := []struct {
		name    string
		trickNo int
		broken  bool
		hand    string
		lead    string
		want    string
	}{
		{"hearts lead once broken", 2, true, "2h 3c", "", "3c 2h"},
		{"only hearts lead before they break", 2, false, "2h 5h", "", "2h 5h"},
		{"the queen of spades may lead before hearts break", 2, false, "Qs 2h", "", "Qs"},
		{"any card when void", 2, false, "Qs 2h 3d", "Kc", "3d Qs 2h"},
		{"points on the first trick when nothing else is held", 1, false, "Qs 2h", "2c", "Qs 2h"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.lead == "" {
				s := tricks(t, tt.trickNo, tt.broken, tt.hand, "4c", "5c", "6c")
				if got := texts(s.View(seat(1)).Legal); got != tt.want {
					t.Errorf("legal %s, want %s", got, tt.want)
				}
				return
			}
			s := tricks(t, tt.trickNo, tt.broken, tt.lead, tt.hand, "5c", "6c")
			s, _ = apply(t, s, seat(1), playing(tt.lead))
			if got := texts(s.View(seat(2)).Legal); got != tt.want {
				t.Errorf("legal %s, want %s", got, tt.want)
			}
		})
	}
}

func TestTheHighestCardOfTheSuitLedTakesTheTrickAndLeads(t *testing.T) {
	s := tricks(t, 2, false, "5d 2c", "Ad 2s", "Qs 3c", "Kd 4c")
	for i, c := range []string{"5d", "Ad", "Qs", "Kd"} {
		s, _ = apply(t, s, seat(i+1), playing(c))
	}
	if s.ToAct()[0] != seat(2) || s.taken[1] != queenPoints || len(s.trick) != 0 || s.trickNo != 3 {
		t.Errorf("to act %v taken %v trick %v no %d", s.ToAct(), s.taken, s.trick, s.trickNo)
	}
	want := &TrickResult{Plays: []Play{{seat(1), hand(t, "5d")[0]}, {seat(2), hand(t, "Ad")[0]}, {seat(3), hand(t, "Qs")[0]}, {seat(4), hand(t, "Kd")[0]}}, Winner: seat(2), Points: queenPoints}
	if diff := cmp.Diff(want, s.View(seat(4)).LastTrick); diff != "" {
		t.Errorf("last trick (-want +got):\n%s", diff)
	}
}

// lastTrick is the hand's thirteenth trick with points already taken.
func lastTrick(t *testing.T, scores, taken []int, hands ...string) state {
	t.Helper()
	s := tricks(t, handSize, true, hands...)
	s.scores, s.taken = scores, taken
	return s
}

func playOut(t *testing.T, s state, cards ...string) (state, []game.Event) {
	t.Helper()
	var all []game.Event
	for i, c := range cards {
		var events []game.Event
		s, events = apply(t, s, seat(i+1), playing(c))
		all = append(all, events...)
	}
	return s, all
}

func TestScoring(t *testing.T) {
	t.Run("each seat scores the points it took", func(t *testing.T) {
		s := lastTrick(t, []int{10, 20, 30, 40}, []int{12, 0, 13, 0}, "Ah", "2c", "3c", "4c")
		next, _ := playOut(t, s, "Ah", "2c", "3c", "4c")
		if !slices.Equal(next.scores, []int{23, 20, 43, 40}) || next.handNo != 2 || next.phase != PhasePass {
			t.Errorf("scores %v hand %d phase %s", next.scores, next.handNo, next.phase)
		}
		if lh := next.View(seat(1)).LastHand; lh == nil || lh.Moon != nil || lh.Points[0].Points != 13 {
			t.Errorf("last hand %+v", lh)
		}
	})
	t.Run("taking every point shoots the moon", func(t *testing.T) {
		s := lastTrick(t, []int{10, 20, 30, 40}, []int{25, 0, 0, 0}, "Ah", "2c", "3c", "4c")
		next, events := playOut(t, s, "Ah", "2c", "3c", "4c")
		if !slices.Equal(next.scores, []int{10, 46, 56, 66}) {
			t.Errorf("scores %v", next.scores)
		}
		scored := events[len(events)-2].(HandScoredEvent)
		if scored.Score.Moon == nil || *scored.Score.Moon != seat(1) {
			t.Errorf("scored %+v", scored)
		}
	})
	t.Run("the match ends at 100 and the lowest score wins", func(t *testing.T) {
		s := lastTrick(t, []int{90, 50, 50, 80}, []int{12, 0, 0, 0}, "Ah", "2c", "3c", "4c")
		next, _ := playOut(t, s, "Ah", "2c", "3c", "4c")
		result, finished := next.Outcome()
		want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat(2), seat(3)}, Reason: ReasonScoreLimit}
		if !finished || !cmp.Equal(result, want) || len(next.ToAct()) != 0 {
			t.Errorf("result %+v finished %t", result, finished)
		}
	})
	t.Run("every seat tied at the end is a draw", func(t *testing.T) {
		s := lastTrick(t, []int{87, 100, 87, 100}, []int{12, 0, 13, 0}, "Ah", "2c", "3c", "4c")
		next, _ := playOut(t, s, "Ah", "2c", "3c", "4c")
		result, finished := next.Outcome()
		want := game.Result{Kind: game.ResultDraw, Reason: ReasonScoreLimit}
		if !finished || !cmp.Equal(result, want) || !slices.Equal(next.scores, []int{100, 100, 100, 100}) {
			t.Errorf("result %+v finished %t scores %v", result, finished, next.scores)
		}
	})
	t.Run("a seat that left cannot win", func(t *testing.T) {
		s := lastTrick(t, []int{90, 10, 50, 80}, []int{12, 0, 0, 0}, "Ah", "2c", "3c", "4c")
		s.out[1] = true
		next, _ := apply(t, s, seat(1), playing("Ah"))
		next, _ = apply(t, next, seat(3), playing("3c"))
		next, _ = apply(t, next, seat(4), playing("4c"))
		if result, _ := next.Outcome(); !slices.Equal(result.Winners, []game.Seat{seat(3)}) {
			t.Errorf("winners %v", result.Winners)
		}
	})
}

func TestASeatThatLeftIsPlayedFor(t *testing.T) {
	t.Run("it passes its three highest cards", func(t *testing.T) {
		s := newGame(t, 4)
		want := highest(s.hands[1])
		next, events := s.Eliminate(seat(2), game.ReasonResign)
		after := next.(state)
		if !slices.Equal(after.passes[1], want) || slices.Contains(after.ToAct(), seat(2)) || len(after.ToAct()) != 3 {
			t.Errorf("passes %v to act %v", after.passes, after.ToAct())
		}
		if diff := cmp.Diff([]game.Event{LeftEvent{By: seat(2)}, PassChosenEvent{By: seat(2), Seat: seat(2)}}, events); diff != "" {
			t.Errorf("events (-want +got):\n%s", diff)
		}
	})
	t.Run("it plays its lowest legal card when its turn comes", func(t *testing.T) {
		s := tricks(t, 2, false, "5d 2c", "Ad 3d 2s", "Qs 3c", "Kd 4c")
		gone, _ := s.Eliminate(seat(2), game.ReasonResign)
		next, events := apply(t, gone.(state), seat(1), playing("5d"))
		if next.ToAct()[0] != seat(3) || texts(next.hands[1]) != "Ad 2s" {
			t.Errorf("to act %v p2 holds %s", next.ToAct(), texts(next.hands[1]))
		}
		if diff := cmp.Diff([]game.Event{PlayedEvent{By: seat(1), Seat: seat(1), Card: hand(t, "5d")[0]}, PlayedEvent{By: seat(1), Seat: seat(2), Card: hand(t, "3d")[0]}}, events); diff != "" {
			t.Errorf("events (-want +got):\n%s", diff)
		}
	})
	t.Run("the last seat left wins", func(t *testing.T) {
		var st game.State[Action, View] = newGame(t, 4)
		for _, n := range []int{1, 2, 3} {
			st, _ = st.Eliminate(seat(n), game.ReasonResign)
		}
		result, finished := st.Outcome()
		want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat(4)}, Reason: game.ReasonResign}
		if !finished || !cmp.Equal(result, want) {
			t.Errorf("result %+v finished %t", result, finished)
		}
	})
}

func TestConcludeOffersNothing(t *testing.T) {
	for _, s := range []state{newGame(t, 1), tricks(t, 2, false, "2c", "3c", "4c", "5c")} {
		imposed := game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
		done := s.Conclude(imposed)
		if got, finished := done.Outcome(); !finished || !cmp.Equal(got, imposed) || len(done.ToAct()) != 0 {
			t.Fatalf("concluded: %+v, finished %t", got, finished)
		}
		for _, at := range s.order {
			if v := done.View(at); len(v.Legal) != 0 || v.MustPass != 0 {
				t.Errorf("concluded view offers %v, must pass %d", v.Legal, v.MustPass)
			}
		}
	}
}

func TestApplyLeavesTheEarlierStateUnchanged(t *testing.T) {
	s := lastTrick(t, []int{10, 20, 30, 40}, []int{12, 0, 0, 0}, "Ah", "2c", "3c", "4c")
	states := []state{s, newGame(t, 1)}
	for _, s := range states {
		before := map[game.Seat]string{}
		for _, at := range s.order {
			before[at] = encoded(t, s.View(at))
		}
		at := s.ToAct()[0]
		next, _ := apply(t, s, at, choose(s.View(at)))
		for len(next.ToAct()) > 0 && next.handNo == s.handNo {
			by := next.ToAct()[0]
			next, _ = apply(t, next, by, choose(next.View(by)))
		}
		for _, at := range s.order {
			if got := encoded(t, s.View(at)); got != before[at] {
				t.Errorf("playing on changed %s's earlier view:\n%s\n%s", at, before[at], got)
			}
		}
	}
}

func TestWireShape(t *testing.T) {
	s := tricks(t, 2, false, "2c 3h", "4c", "5c", "6c")
	s.received[0] = hand(t, "3h")
	want := `{"hand_no":1,"pass":"left","phase":"play","score_limit":100,"seats":[` +
		`{"seat":"p1","score":0,"taken":0,"pass_chosen":false,"out":false},{"seat":"p2","score":0,"taken":0,"pass_chosen":false,"out":false},` +
		`{"seat":"p3","score":0,"taken":0,"pass_chosen":false,"out":false},{"seat":"p4","score":0,"taken":0,"pass_chosen":false,"out":false}],` +
		`"trick":[],"last_trick":null,"hearts_broken":false,"my_hand":["2c","3h"],"legal":["2c"],"must_pass":0,"pass_to":"p2",` +
		`"passed":[],"received":["3h"],"last_hand":null}`
	if got := encoded(t, s.View(seat(1))); got != want {
		t.Errorf("view\n got %s\nwant %s", got, want)
	}
}

func TestCataloguePatternsAreTheParsersOwn(t *testing.T) {
	actions := (Rules{}).Info().Actions
	if actions[0].Input.Pattern != passSyntax.String() || actions[1].Input.Pattern != playSyntax.String() {
		t.Errorf("catalogue patterns %q %q", actions[0].Input.Pattern, actions[1].Input.Pattern)
	}
}
