package poker

import (
	"slices"
	"strings"
	"testing"
)

// street opens a postflop round with a 20 big blind for seats with these stacks.
func street(stacks ...Chips) betting {
	b := betting{n: len(stacks), bigBlind: 20}
	for i, stack := range stacks {
		b.stack[i], b.contesting[i] = stack, true
	}
	return b.newStreet()
}

// moves plays "0:bet 100, 1:allin" in order and returns the error of the last move.
func moves(t *testing.T, b betting, script string) (betting, error) {
	t.Helper()
	steps := strings.Split(script, ",")
	for k, step := range steps {
		seatText, text, _ := strings.Cut(strings.TrimSpace(step), ":")
		seat := int(seatText[0] - '0')
		m, err := parseMove(text)
		if err != nil {
			t.Fatalf("%q: %v", step, err)
		}
		next, _, err := b.apply(seat, m)
		if err != nil {
			if k == len(steps)-1 {
				return b, err
			}
			t.Fatalf("%q: %v", step, err)
		}
		b = next
	}
	return b, nil
}

func TestBettingRejectsIllegalSizesAndMoves(t *testing.T) {
	tests := []struct {
		name   string
		stacks []Chips
		script string
		want   string
	}{
		{"bet below the big blind", []Chips{1000, 1000}, "0:bet 10", "the minimum bet is 20"},
		{"bet into an open bet", []Chips{1000, 1000}, "0:bet 100, 1:bet 300", "a bet is already open"},
		{"raise with no bet", []Chips{1000, 1000}, "0:raise 100", "no bet to raise"},
		{"check facing a bet", []Chips{1000, 1000}, "0:bet 100, 1:check", "cannot check: 100 to call"},
		{"raise below the last full increment", []Chips{1000, 1000, 1000}, "0:bet 100, 1:raise 350, 2:raise 550", "the minimum raise is 600"},
		{"raise beyond the stack", []Chips{1000, 300}, "0:bet 100, 1:raise 400", "raise 400 is more than your 300"},
		{"raise after only an incomplete all-in", []Chips{1000, 150, 1000}, "0:bet 100, 1:allin, 2:call, 0:raise 400", "not reopened"},
		{"allin over the bet after only an incomplete all-in", []Chips{1000, 150, 1000}, "0:bet 100, 1:allin, 2:call, 0:allin", "not reopened"},
		{"raise when everyone else is all-in", []Chips{500, 1000}, "0:allin, 1:raise 1000", "everyone else is all-in"},
		{"allin over an all-in no one else can call", []Chips{500, 1000}, "0:allin, 1:allin", "everyone else is all-in"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := moves(t, street(tt.stacks...), tt.script)
			wantIllegal(t, err, tt.want)
		})
	}
}

func TestBettingSizesRaisesFromTheLastFullIncrement(t *testing.T) {
	tests := []struct {
		name           string
		stacks         []Chips
		script         string
		wantCurrentBet Chips
		wantMinRaiseTo Chips
	}{
		{"opening bet sets the increment", []Chips{1000, 1000}, "0:bet 100", 100, 200},
		{"a bigger raise grows it", []Chips{1000, 1000, 1000}, "0:bet 100, 1:raise 350", 350, 600},
		{"an incomplete all-in raises the bet but not the increment", []Chips{1000, 150, 1000}, "0:bet 100, 1:allin", 150, 250},
		{"an all-in below the big blind opens without changing it", []Chips{1000, 15}, "0:check, 1:allin", 15, 35},
		{"an all-in call for less leaves the bet", []Chips{1000, 60}, "0:bet 100, 1:allin", 100, 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := moves(t, street(tt.stacks...), tt.script)
			if err != nil {
				t.Fatal(err)
			}
			if b.currentBet != tt.wantCurrentBet || b.minRaiseTo() != tt.wantMinRaiseTo {
				t.Errorf("bet %d, min raise to %d; want %d, %d", b.currentBet, b.minRaiseTo(), tt.wantCurrentBet, tt.wantMinRaiseTo)
			}
		})
	}
}

func TestLegalMovesFollowTheReopenRuleAndTheStack(t *testing.T) {
	tests := []struct {
		name   string
		stacks []Chips
		script string
		seat   int
		want   []MoveKind
	}{
		{"no bet open: check or bet", []Chips{1000, 1000}, "0:check", 1, []MoveKind{MoveFold, MoveCheck, MoveBet, MoveAllIn}},
		{"a seat yet to act may raise an incomplete all-in", []Chips{1000, 150, 1000}, "0:bet 100, 1:allin", 2, []MoveKind{MoveFold, MoveCall, MoveRaise, MoveAllIn}},
		{"the bettor facing only an incomplete all-in may not", []Chips{1000, 150, 1000}, "0:bet 100, 1:allin, 2:call", 0, []MoveKind{MoveFold, MoveCall}},
		{"a full raise after the incomplete one reopens", []Chips{1000, 150, 1000}, "0:bet 100, 1:allin, 2:raise 250", 0, []MoveKind{MoveFold, MoveCall, MoveRaise, MoveAllIn}},
		{"incomplete all-ins that add up to a full raise reopen", []Chips{1000, 150, 220, 1000}, "0:bet 100, 1:allin, 2:allin, 3:call", 0, []MoveKind{MoveFold, MoveCall, MoveRaise, MoveAllIn}},
		{"a checker facing an all-in below the big blind may not", []Chips{1000, 15, 1000}, "0:check, 1:allin, 2:call", 0, []MoveKind{MoveFold, MoveCall}},
		{"a stack short of the minimum raise may still go all-in", []Chips{1000, 150}, "0:bet 100", 1, []MoveKind{MoveFold, MoveCall, MoveAllIn}},
		{"facing an all-in no one else can call: call or fold", []Chips{500, 1000}, "0:allin", 1, []MoveKind{MoveFold, MoveCall}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := moves(t, street(tt.stacks...), tt.script)
			if err != nil {
				t.Fatal(err)
			}
			if !b.needsAction(tt.seat) {
				t.Fatalf("seat %d does not need to act", tt.seat)
			}
			if got := b.legalMoves(tt.seat); !slices.Equal(got, tt.want) {
				t.Errorf("legalMoves(%d) = %v, want %v", tt.seat, got, tt.want)
			}
		})
	}
}

func TestBettingRoundEnds(t *testing.T) {
	tests := []struct {
		name   string
		stacks []Chips
		script string
		want   bool
	}{
		{"everyone checked", []Chips{1000, 1000, 1000}, "0:check, 1:check, 2:check", true},
		{"one seat has not acted", []Chips{1000, 1000, 1000}, "0:check, 1:check", false},
		{"a bet is called round", []Chips{1000, 1000, 1000}, "0:bet 100, 1:call, 2:call", true},
		{"a raise sends action back", []Chips{1000, 1000, 1000}, "0:bet 100, 1:call, 2:raise 200", false},
		{"an incomplete all-in still needs a call", []Chips{1000, 150, 1000}, "0:bet 100, 1:allin, 2:call", false},
		{"the last seat with chips calls an all-in", []Chips{500, 1000}, "0:allin, 1:call", true},
		{"call with nothing to call checks", []Chips{1000, 1000}, "0:call, 1:call", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := moves(t, street(tt.stacks...), tt.script)
			if err != nil {
				t.Fatal(err)
			}
			if _, pending := b.nextToAct(0); pending == tt.want {
				t.Errorf("round over = %t, want %t", !pending, tt.want)
			}
		})
	}
}

func TestToCallNeverExceedsWhatAnOpponentCanPutIn(t *testing.T) {
	b := street(1000, 5)
	b, _ = b.post(0, 10)
	b, _ = b.post(1, 20)
	b.currentBet = 20
	if got := b.toCall(0); got != 0 {
		t.Errorf("toCall = %d facing a 5-chip all-in big blind, want 0", got)
	}
	if b.needsAction(0) {
		t.Error("the small blind must not act when the only opponent is all-in for less")
	}
}
