package uno

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

func TestTheDeckIsTheOfficial108Cards(t *testing.T) {
	deck := orderedDeck()
	count := map[string]int{}
	for _, c := range deck {
		count[c.String()]++
	}
	want := map[string]int{"R0": 1, "R7": 2, "YS": 2, "GR": 2, "B+2": 2, "W": 4, "W+4": 4}
	for text, n := range want {
		if count[text] != n {
			t.Errorf("%s: %d copies, want %d", text, count[text], n)
		}
	}
	if len(deck) != 108 || len(count) != 54 {
		t.Errorf("%d cards of %d kinds, want 108 of 54", len(deck), len(count))
	}
}

func TestACardDecodesFromItsWireFormAlone(t *testing.T) {
	for _, c := range orderedDeck() {
		var got Card
		if err := got.UnmarshalText([]byte(c.String())); err != nil || got != c {
			t.Errorf("%s decodes as %+v, %v", c, got, err)
		}
	}
	for _, text := range []string{"", "R", "r5", "RW", "RW+4", "W+2", "X5", "R10", "R5 "} {
		var got Card
		if err := got.UnmarshalText([]byte(text)); err == nil {
			t.Errorf("%q decodes as %+v, want no card", text, got)
		}
	}
}

func TestNewDealsSevenEachAndTurnsUpACardThatIsNoWildDrawFour(t *testing.T) {
	for seed := range game.Seed(40) {
		s := newGame(t, 4, seed)
		dealt := 4 * handSize
		if s.top().Face == DrawTwo {
			dealt += drawTwoCards
		}
		held := 0
		for _, h := range s.hands {
			held += len(h)
		}
		if held != dealt || held+len(s.pile)+len(s.discard) != len(orderedDeck()) || s.top().Face == WildDrawFour {
			t.Fatalf("seed %d: %d cards held, %d in the piles, top %s", seed, held, len(s.pile)+len(s.discard), s.top())
		}
	}
}

// deckFor is a deck that deals each of n seats, from the dealer's left, the
// given hand and then turns up top.
func deckFor(t *testing.T, top string, hands ...string) []Card {
	t.Helper()
	parsed := make([][]Card, len(hands))
	for i, h := range hands {
		parsed[i] = hand(t, h)
	}
	var deck []Card
	for k := range handSize {
		for _, h := range parsed {
			deck = append(deck, h[k])
		}
	}
	return append(deck, hand(t, top+" R1 R2 R3")...)
}

func TestTheCardTurnedUpActsAsIfTheDealerPlayedIt(t *testing.T) {
	const seven = "R1 R2 R3 R4 R5 R6 R7"
	tests := []struct {
		name     string
		top      string
		players  int
		toAct    game.Seat
		phase    Phase
		dir      Direction
		drewFrom game.Seat
	}{
		{"a number: left of the dealer starts", "G5", 3, seat(2), PhasePlay, Clockwise, ""},
		{"a skip skips left of the dealer", "GS", 3, seat(3), PhasePlay, Clockwise, ""},
		{"a reverse: the dealer starts, to the right", "GR", 3, seat(1), PhasePlay, CounterClockwise, ""},
		{"a draw two: left of the dealer draws two and is skipped", "G+2", 3, seat(3), PhasePlay, Clockwise, seat(2)},
		{"a wild: left of the dealer names the colour", "W", 3, seat(2), PhaseColor, Clockwise, ""},
		{"heads-up reverse: the dealer starts", "GR", 2, seat(1), PhasePlay, CounterClockwise, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hands := slices.Repeat([]string{seven}, tt.players)
			order, _ := game.NumberSeats(gametest.Users(tt.players))
			s, events := dealFrom(order, 0, 1, deckFor(t, tt.top, hands...))
			if got := s.ToAct(); !slices.Equal(got, []game.Seat{tt.toAct}) || s.phase != tt.phase || s.direction() != tt.dir {
				t.Errorf("to act %v phase %s %s, want %s %s %s", got, s.phase, s.direction(), tt.toAct, tt.phase, tt.dir)
			}
			if started := events[0].(StartedEvent); started.Top.String() != tt.top || started.Dealer != seat(1) {
				t.Errorf("started %+v", started)
			}
			for i, h := range s.hands {
				want := handSize
				if s.order[i] == tt.drewFrom {
					want += drawTwoCards
				}
				if len(h) != want {
					t.Errorf("%s holds %d, want %d", s.order[i], len(h), want)
				}
			}
		})
	}
}

func TestAWildDrawFourTurnedUpGoesBackForAnotherCard(t *testing.T) {
	order, _ := game.NumberSeats(gametest.Users(2))
	deck := deckFor(t, "W+4", "R1 R2 R3 R4 R5 R6 R7", "G1 G2 G3 G4 G5 G6 G7")
	s, _ := dealFrom(order, 0, 1, deck)
	if s.top().Face == WildDrawFour || len(s.pile)+len(s.discard) != 4 || s.shuffles == 0 {
		t.Errorf("top %s, %d cards left, %d shuffles", s.top(), len(s.pile)+len(s.discard), s.shuffles)
	}
}

func TestPlayRejects(t *testing.T) {
	s := table(t, "R5", "G1", "R7 Y5 B9 W W+4 G+2", "Y1")
	tests := []struct {
		name string
		by   game.Seat
		a    Action
		want string
	}{
		{"a card not held", seat(1), playing("r9"), "you hold no R9"},
		{"a card that matches neither colour nor face", seat(1), playing("b9"), "B9 does not match the R colour or the R5 on the pile"},
		{"another colour's action card", seat(1), playing("g+2"), "G+2 does not match the R colour or the R5 on the pile"},
		{"a wild with no colour", seat(1), playing("w"), `"w" is no card to play`},
		{"a colour on a coloured card", seat(1), playing("r7 g"), `"r7 g" is no card to play`},
		{"text that is no card", seat(1), playing("red seven"), `"red seven" is no card to play`},
		{"passing without drawing", seat(1), kind(ActionPass), "you cannot pass now: play a card or draw"},
		{"challenging with nothing to challenge", seat(1), kind(ActionChallenge), "you cannot challenge now"},
		{"naming a colour mid-hand", seat(1), Action{Kind: ActionColor, Color: "R"}, "you cannot color now"},
		{"an unknown kind", seat(1), kind("shout"), "you cannot shout now"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := s.Apply(tt.by, tt.a, nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err %v, want %q", err, tt.want)
			}
			if _, ok := errors.AsType[game.IllegalActionError](err); !ok {
				t.Errorf("err %T, want an IllegalActionError", err)
			}
		})
	}
}

func TestAMatchingCardGoesOnThePileAndSetsTheColour(t *testing.T) {
	tests := []struct {
		name  string
		top   string
		color Color
		play  string
		want  Color
		toAct game.Seat
	}{
		{"same colour", "R5", Red, "r7", Red, seat(2)},
		{"same number", "R5", Red, "y5", Yellow, seat(2)},
		{"same action", "RS", Red, "gs", Green, seat(3)},
		{"a wild names its colour", "R5", Red, "w b", Blue, seat(2)},
		{"a wild on a wild", "W", Green, "w y", Yellow, seat(2)},
		{"the colour a wild named", "W", Green, "g1", Green, seat(2)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := table(t, tt.top, "B1", "R7 Y5 GS G1 W B3", "Y1 Y2", "Y3 Y4")
			s.color = tt.color
			next, events := apply(t, s, seat(1), playing(tt.play))
			played := events[0].(PlayedEvent)
			if next.color != tt.want || played.Color != tt.want || next.top() != played.Card || len(next.hands[0]) != 5 {
				t.Errorf("colour %s, event %+v, top %s, %d left", next.color, played, next.top(), len(next.hands[0]))
			}
			if got := next.ToAct(); !slices.Equal(got, []game.Seat{tt.toAct}) {
				t.Errorf("to act %v, want %s", got, tt.toAct)
			}
		})
	}
}

func TestActionCards(t *testing.T) {
	tests := []struct {
		name    string
		players int
		play    string
		toAct   game.Seat
		dir     Direction
		p2Cards int
	}{
		{"skip passes over the next seat", 3, "rs", seat(3), Clockwise, 2},
		{"reverse turns play to the previous seat", 3, "rr", seat(3), CounterClockwise, 2},
		{"heads-up reverse acts as a skip", 2, "rr", seat(1), CounterClockwise, 2},
		{"draw two makes the next seat draw two and lose its turn", 3, "r+2", seat(3), Clockwise, 4},
		{"heads-up skip plays again", 2, "rs", seat(1), Clockwise, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hands := []string{"RS RR R+2 B1", "Y1 Y2", "Y3 Y4"}[:tt.players]
			s := table(t, "R5", "G1 G2 G3", hands...)
			next, _ := apply(t, s, seat(1), playing(tt.play))
			if got := next.ToAct(); !slices.Equal(got, []game.Seat{tt.toAct}) || next.direction() != tt.dir || len(next.hands[1]) != tt.p2Cards {
				t.Errorf("to act %v %s, p2 holds %d; want %s %s %d", got, next.direction(), len(next.hands[1]), tt.toAct, tt.dir, tt.p2Cards)
			}
		})
	}
}

func TestDrawing(t *testing.T) {
	t.Run("a card that matches may be played at once, and only it", func(t *testing.T) {
		s := table(t, "R5", "R9 G1", "R7 B1", "Y1")
		drew, events := apply(t, s, seat(1), kind(ActionDraw))
		if drew.phase != PhaseDrawn || drew.ToAct()[0] != seat(1) || events[0] != (DrewEvent{By: seat(1), Seat: seat(1), Count: 1}) {
			t.Fatalf("phase %s to act %v events %v", drew.phase, drew.ToAct(), events)
		}
		if v := drew.View(seat(1)); !slices.Equal(v.Playable, hand(t, "R9")) || !slices.Equal(v.Legal, []game.ActionKind{ActionPlay, ActionPass}) {
			t.Errorf("drawn view offers %v %v", v.Legal, v.Playable)
		}
		if _, _, err := drew.Apply(seat(1), playing("r7"), nil); err == nil || !strings.Contains(err.Error(), "only the card you drew, R9") {
			t.Errorf("playing another card after drawing: %v", err)
		}
		played, _ := apply(t, drew, seat(1), playing("r9"))
		kept, keptEvents := apply(t, drew, seat(1), kind(ActionPass))
		if played.top().String() != "R9" || played.ToAct()[0] != seat(2) || kept.ToAct()[0] != seat(2) || len(kept.hands[0]) != 3 {
			t.Errorf("played top %s to act %v; kept to act %v holding %d", played.top(), played.ToAct(), kept.ToAct(), len(kept.hands[0]))
		}
		if keptEvents[0] != (PassedEvent{By: seat(1)}) {
			t.Errorf("keeping the card: %v", keptEvents)
		}
	})
	t.Run("a card that does not match is kept, as a matching one may be", func(t *testing.T) {
		s := table(t, "R5", "G9 G1", "R7 B1", "Y1")
		drew, _ := apply(t, s, seat(1), kind(ActionDraw))
		if drew.phase != PhaseDrawn || drew.ToAct()[0] != seat(1) || len(drew.hands[0]) != 3 {
			t.Fatalf("phase %s to act %v holding %d", drew.phase, drew.ToAct(), len(drew.hands[0]))
		}
		if v := drew.View(seat(1)); len(v.Playable) != 0 || !slices.Equal(v.Legal, []game.ActionKind{ActionPass}) {
			t.Errorf("drawn view offers %v %v", v.Legal, v.Playable)
		}
		if _, _, err := drew.Apply(seat(1), playing("g9"), nil); err == nil || !strings.Contains(err.Error(), "does not match") {
			t.Errorf("playing the card drawn: %v", err)
		}
		kept, events := apply(t, drew, seat(1), kind(ActionPass))
		if kept.ToAct()[0] != seat(2) || len(events) != 1 || events[0] != (PassedEvent{By: seat(1)}) {
			t.Errorf("kept to act %v events %v", kept.ToAct(), events)
		}
	})
	t.Run("an empty draw pile is the discard pile under its top, shuffled", func(t *testing.T) {
		s := table(t, "R5", "", "R7 B1", "Y1")
		s.discard = hand(t, "G1 G2 G3 R5")
		drew, events := apply(t, s, seat(1), kind(ActionDraw))
		if drew.top().String() != "R5" || len(drew.discard) != 1 || len(drew.pile) != 2 || events[0] != (ReshuffledEvent{By: seat(1), Cards: 3}) {
			t.Errorf("top %s discard %v pile %v events %v", drew.top(), drew.discard, drew.pile, events)
		}
	})
	t.Run("with nothing left to draw the turn passes", func(t *testing.T) {
		s := table(t, "R5", "", "B7 B1", "Y1")
		drew, events := apply(t, s, seat(1), kind(ActionDraw))
		if drew.ToAct()[0] != seat(2) || len(drew.hands[0]) != 2 || events[0] != (DrewEvent{By: seat(1), Seat: seat(1), Count: 0}) {
			t.Errorf("to act %v holding %d events %v", drew.ToAct(), len(drew.hands[0]), events)
		}
	})
}

func TestWildDrawFour(t *testing.T) {
	// p1 plays a Wild Draw Four over red, naming blue; it still holds R7 in the guilty case.
	tests := []struct {
		name      string
		p1        string
		answer    game.ActionKind
		guilty    bool
		p1Cards   int
		p2Cards   int
		toAct     game.Seat
		challenge bool
	}{
		{"accepted: p2 draws four and loses its turn", "W+4 R7 B1", ActionAccept, false, 2, 6, seat(3), false},
		{"challenged and guilty: p1 draws four, p2 plays on", "W+4 R7 B1", ActionChallenge, true, 6, 2, seat(2), true},
		{"challenged and innocent: p2 draws six and loses its turn", "W+4 G7 B1", ActionChallenge, false, 2, 8, seat(3), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := table(t, "R5", "G1 G2 G3 G4 G5 G6 G7 G8", tt.p1, "Y1 Y2", "Y3 Y4")
			facing, _ := apply(t, s, seat(1), playing("w+4 b"))
			if facing.phase != PhaseChallenge || facing.ToAct()[0] != seat(2) || facing.color != Blue {
				t.Fatalf("phase %s to act %v colour %s", facing.phase, facing.ToAct(), facing.color)
			}
			if v := facing.View(seat(2)); !slices.Equal(v.Legal, []game.ActionKind{ActionChallenge, ActionAccept}) || len(v.Playable) != 0 {
				t.Errorf("facing view offers %v %v", v.Legal, v.Playable)
			}
			next, events := apply(t, facing, seat(2), kind(tt.answer))
			if tt.challenge && events[0] != (ChallengedEvent{By: seat(2), Seat: seat(1), Guilty: tt.guilty}) {
				t.Errorf("events %v", events)
			}
			if len(next.hands[0]) != tt.p1Cards || len(next.hands[1]) != tt.p2Cards || next.ToAct()[0] != tt.toAct || next.phase != PhasePlay || next.color != Blue {
				t.Errorf("p1 %d p2 %d to act %v phase %s colour %s", len(next.hands[0]), len(next.hands[1]), next.ToAct(), next.phase, next.color)
			}
		})
	}
}

func TestPlayingTheLastCardWinsAtOnce(t *testing.T) {
	s := table(t, "R5", "G1 G2", "R+2", "Y1")
	next, _ := apply(t, s, seat(1), playing("r+2"))
	result, finished := next.Outcome()
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat(1)}, Reason: ReasonWentOut}
	if !finished || !cmp.Equal(result, want) || len(next.ToAct()) != 0 || len(next.hands[1]) != 1 {
		t.Errorf("result %+v finished %t to act %v; p2 holds %d", result, finished, next.ToAct(), len(next.hands[1]))
	}
	if _, _, err := next.Apply(seat(2), playing("y1"), nil); err == nil {
		t.Error("a finished game took a move")
	}
}

func TestEliminate(t *testing.T) {
	t.Run("the seat to act leaving passes the turn on", func(t *testing.T) {
		s := table(t, "R5", "G1", "R1", "Y1", "Y2")
		next, events := s.Eliminate(seat(1), game.ReasonResign)
		after := next.(state)
		if after.ToAct()[0] != seat(2) || len(after.hands[0]) != 0 || !after.out[0] || events[0] != (LeftEvent{By: seat(1)}) {
			t.Errorf("to act %v hands %v out %v events %v", after.ToAct(), after.hands, after.out, events)
		}
		if v := after.View(seat(3)); !v.Seats[0].Out || v.Seats[0].Cards != 0 {
			t.Errorf("seats %+v", v.Seats)
		}
	})
	t.Run("a Wild Draw Four whose player left is void", func(t *testing.T) {
		s := table(t, "R5", "G1", "W+4 R1", "Y1", "Y2")
		facing, _ := apply(t, s, seat(1), playing("w+4 g"))
		next, _ := facing.Eliminate(seat(1), game.ReasonResign)
		after := next.(state)
		if after.phase != PhasePlay || after.ToAct()[0] != seat(2) || len(after.hands[1]) != 1 {
			t.Errorf("phase %s to act %v p2 holds %d", after.phase, after.ToAct(), len(after.hands[1]))
		}
	})
	t.Run("a seat facing a Wild Draw Four leaving passes the turn on", func(t *testing.T) {
		s := table(t, "R5", "G1", "W+4 R1", "Y1", "Y2")
		facing, _ := apply(t, s, seat(1), playing("w+4 g"))
		next, _ := facing.Eliminate(seat(2), game.ReasonResign)
		if after := next.(state); after.phase != PhasePlay || after.ToAct()[0] != seat(3) {
			t.Errorf("phase %s to act %v", after.phase, after.ToAct())
		}
	})
	t.Run("the last seat left wins", func(t *testing.T) {
		s := table(t, "R5", "G1", "R1", "Y1", "Y2")
		once, _ := s.Eliminate(seat(2), game.ReasonResign)
		twice, _ := once.Eliminate(seat(1), game.ReasonResign)
		result, finished := twice.Outcome()
		want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat(3)}, Reason: game.ReasonResign}
		if !finished || !cmp.Equal(result, want) {
			t.Errorf("result %+v finished %t", result, finished)
		}
	})
}

func TestConcludeOffersNothing(t *testing.T) {
	s := table(t, "R5", "G1", "R1 R2", "Y1")
	imposed := game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	done := s.Conclude(imposed)
	if got, finished := done.Outcome(); !finished || !cmp.Equal(got, imposed) || len(done.ToAct()) != 0 {
		t.Fatalf("concluded: %+v, finished %t", got, finished)
	}
	if v := done.View(seat(1)); len(v.Legal) != 0 || len(v.Playable) != 0 {
		t.Errorf("concluded view offers %v %v", v.Legal, v.Playable)
	}
}

func TestApplyLeavesTheEarlierStateUnchanged(t *testing.T) {
	s := table(t, "R5", "G1 G2 G3", "R7 R+2 W+4 RS", "Y1 Y2", "Y3")
	before := map[game.Seat]string{}
	for _, at := range s.order {
		before[at] = gametest.ViewJSON(t, s.View(at))
	}
	for _, a := range []Action{playing("r7"), playing("r+2"), playing("w+4 b"), playing("rs"), kind(ActionDraw)} {
		next, _ := apply(t, s, seat(1), a)
		if next.phase == PhaseChallenge {
			next, _ = apply(t, next, seat(2), kind(ActionChallenge))
		}
		for _, at := range s.order {
			if got := gametest.ViewJSON(t, s.View(at)); got != before[at] {
				t.Errorf("%+v changed %s's earlier view:\n%s\n%s", a, at, before[at], got)
			}
		}
	}
}

func TestWireShape(t *testing.T) {
	s := table(t, "R5", "G1 G2", "R7 W+4 B1", "Y1")
	want := `{"top":"R5","color":"R","direction":"clockwise","phase":"play","draw_pile":2,` +
		`"seats":[{"seat":"p1","cards":3,"out":false},{"seat":"p2","cards":1,"out":false}],` +
		`"my_hand":["R7","B1","W+4"],"legal":["play","draw"],"playable":["R7","W+4"]}`
	if got := gametest.ViewJSON(t, s.View(seat(1))); got != want {
		t.Errorf("view\n got %s\nwant %s", got, want)
	}
	waiting := `{"top":"R5","color":"R","direction":"clockwise","phase":"play","draw_pile":2,` +
		`"seats":[{"seat":"p1","cards":3,"out":false},{"seat":"p2","cards":1,"out":false}],` +
		`"my_hand":["Y1"],"legal":[],"playable":[]}`
	if got := gametest.ViewJSON(t, s.View(seat(2))); got != waiting {
		t.Errorf("waiting view\n got %s\nwant %s", got, waiting)
	}
}

func TestCataloguePatternIsTheParsersOwn(t *testing.T) {
	if got := (Rules{}).Info().Actions[0].Input.Pattern; got != playSyntax.String() {
		t.Errorf("catalogue pattern %q, parser %q", got, playSyntax.String())
	}
}
