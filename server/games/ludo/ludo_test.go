package ludo

import (
	"encoding/json"
	"errors"
	"maps"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const (
	out  = ProgressBase
	home = ProgressHome
)

func inBase() Tokens { return Tokens{out, out, out, out} }

// position is a state where turn has rolled roll; phase follows from whether it is zero.
func position(turn game.Seat, roll Roll, tokens map[game.Seat]Tokens) state {
	phase := PhaseMove
	if roll == 0 {
		phase = PhaseRoll
	}
	return state{order: clockwise(slices.Collect(maps.Keys(tokens))), tokens: tokens, turn: turn, phase: phase, lastRoll: roll}
}

// rngRolling is an rng whose next die throw is roll.
func rngRolling(t *testing.T, roll Roll) *rand.Rand {
	t.Helper()
	for seq := range game.Seq(1000) {
		if rollDie(game.StepRNG(1, seq)) == roll {
			return game.StepRNG(1, seq)
		}
	}
	t.Fatalf("no rng rolls %d", roll)
	return nil
}

func mustApply(t *testing.T, s state, action Action, rng *rand.Rand) (state, []game.Event) {
	t.Helper()
	next, events, err := s.Apply(s.turn, action, rng)
	if err != nil {
		t.Fatalf("Apply(%+v): %v", action, err)
	}
	return next.(state), events
}

func TestNewSeatsInColourOrderAndTurnsGoClockwiseFromADrawnFirstSeat(t *testing.T) {
	two := map[game.UserID]game.Seat{"alice": SeatBlue, "bob": SeatYellow}
	three := map[game.UserID]game.Seat{"alice": SeatBlue, "bob": SeatYellow, "carol": SeatRed}
	four := map[game.UserID]game.Seat{"alice": SeatBlue, "bob": SeatYellow, "carol": SeatRed, "dave": SeatGreen}
	tests := []struct {
		invitees  []game.UserID
		seed      game.Seed
		wantSeats map[game.UserID]game.Seat
		wantTurns []game.Seat
	}{
		{[]game.UserID{"bob"}, 1, two, []game.Seat{SeatYellow, SeatBlue, SeatYellow}},
		{[]game.UserID{"bob"}, 6, two, []game.Seat{SeatBlue, SeatYellow, SeatBlue}},
		{[]game.UserID{"bob", "carol"}, 1, three, []game.Seat{SeatYellow, SeatBlue, SeatRed, SeatYellow}},
		{[]game.UserID{"bob", "carol"}, 6, three, []game.Seat{SeatBlue, SeatRed, SeatYellow, SeatBlue}},
		{[]game.UserID{"bob", "carol", "dave"}, 1, four, []game.Seat{SeatGreen, SeatBlue, SeatRed, SeatYellow, SeatGreen}},
		{[]game.UserID{"bob", "carol", "dave"}, 6, four, []game.Seat{SeatRed, SeatYellow, SeatGreen, SeatBlue, SeatRed}},
	}
	for _, tt := range tests {
		st, seats, err := Rules{}.New(Opts{}, "alice", tt.invitees, game.StepRNG(tt.seed, 0))
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(tt.wantSeats, seats); diff != "" {
			t.Errorf("seats (-want +got):\n%s", diff)
		}
		s := st.(state)
		var turns []game.Seat
		for range tt.wantTurns {
			turns = append(turns, s.ToAct()[0])
			s = s.passTurn()
		}
		if diff := cmp.Diff(tt.wantTurns, turns); diff != "" {
			t.Errorf("%d players, seed %d: turn order (-want +got):\n%s", len(seats), tt.seed, diff)
		}
	}
}

func TestViewStartsCoverEverySeatInPlay(t *testing.T) {
	s := position(SeatBlue, 0, map[game.Seat]Tokens{SeatBlue: inBase(), SeatYellow: inBase(), SeatRed: inBase()})
	want := map[game.Seat]int{SeatBlue: 0, SeatRed: 13, SeatYellow: 26}
	if diff := cmp.Diff(want, s.View(SeatBlue).Starts); diff != "" {
		t.Errorf("starts (-want +got):\n%s", diff)
	}
	after, _ := s.Eliminate(SeatRed, game.ReasonResign)
	delete(want, SeatRed)
	if diff := cmp.Diff(want, after.View(SeatBlue).Starts); diff != "" {
		t.Errorf("starts after red resigns (-want +got):\n%s", diff)
	}
}

func TestThreePlayers(t *testing.T) {
	tests := []struct {
		name         string
		tokens       map[game.Seat]Tokens
		roll         Roll
		wantMovable  []Token
		wantProgress map[game.Seat]Tokens
		wantEvents   []game.Event
	}{
		{
			name:         "blue captures red's lone token with yellow in play",
			tokens:       map[game.Seat]Tokens{SeatBlue: {10, out, out, out}, SeatRed: {1, out, out, out}, SeatYellow: {30, out, out, out}},
			roll:         4,
			wantMovable:  []Token{1},
			wantProgress: map[game.Seat]Tokens{SeatBlue: {14, out, out, out}, SeatRed: inBase(), SeatYellow: {30, out, out, out}},
			wantEvents: []game.Event{
				MovedEvent{By: SeatBlue, Token: 1, From: 10, To: 14},
				CapturedEvent{By: SeatBlue, Victim: SeatRed, Token: 1},
			},
		},
		{
			name:         "red's pair blocks blue, who captures yellow's lone token instead",
			tokens:       map[game.Seat]Tokens{SeatBlue: {10, 3, out, out}, SeatRed: {1, 1, out, out}, SeatYellow: {33, out, out, out}},
			roll:         4,
			wantMovable:  []Token{2},
			wantProgress: map[game.Seat]Tokens{SeatBlue: {10, 7, out, out}, SeatRed: {1, 1, out, out}, SeatYellow: inBase()},
			wantEvents: []game.Event{
				MovedEvent{By: SeatBlue, Token: 2, From: 3, To: 7},
				CapturedEvent{By: SeatBlue, Victim: SeatYellow, Token: 1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := position(SeatBlue, tt.roll, tt.tokens)
			movable := s.View(SeatBlue).Movable
			if diff := cmp.Diff(tt.wantMovable, movable); diff != "" {
				t.Fatalf("movable (-want +got):\n%s", diff)
			}
			next, events := mustApply(t, s, Action{Kind: ActionMove, Token: movable[len(movable)-1]}, nil)
			if diff := cmp.Diff(tt.wantEvents, events); diff != "" {
				t.Errorf("events (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantProgress, next.View(SeatBlue).Progress); diff != "" {
				t.Errorf("progress (-want +got):\n%s", diff)
			}
			if next.turn != SeatRed {
				t.Errorf("turn %s, want red next clockwise", next.turn)
			}
		})
	}
}

func TestEliminatingASeatNotToActKeepsTheTurn(t *testing.T) {
	s := position(SeatBlue, 4, map[game.Seat]Tokens{SeatBlue: {3, out, out, out}, SeatRed: {7, out, out, out}, SeatYellow: {9, out, out, out}})
	st, _ := s.Eliminate(SeatRed, game.ReasonTimeout)
	next := st.(state)
	view := next.View(SeatBlue)
	if next.turn != SeatBlue || view.Phase != PhaseMove || view.LastRoll != 4 || !slices.Equal(view.Movable, []Token{1}) {
		t.Errorf("turn %s view %+v, want blue still to move its token by 4", next.turn, view)
	}
	if _, ok := view.Progress[SeatRed]; ok {
		t.Error("red's tokens remain on the board")
	}
	if after, _ := mustApply(t, next, Action{Kind: ActionMove, Token: 1}, nil); after.turn != SeatYellow {
		t.Errorf("turn after blue's move is %s, want yellow, skipping red", after.turn)
	}
}

func TestTwoPlayersStartOpposite(t *testing.T) {
	if got := trackStart(SeatYellow) - trackStart(SeatBlue); got != TrackLength/2 {
		t.Errorf("blue and yellow starts are %d apart, want %d", got, TrackLength/2)
	}
}

func TestNewRejectsPlayerCount(t *testing.T) {
	for _, invitees := range [][]game.UserID{nil, {"b", "c", "d", "e"}} {
		_, _, err := Rules{}.New(Opts{}, "a", invitees, game.StepRNG(1, 0))
		if !errors.As(err, new(game.BadRequestError)) {
			t.Errorf("%d invitees: got %v, want a bad request", len(invitees), err)
		}
	}
}

func TestRoll(t *testing.T) {
	tests := []struct {
		name        string
		blue        Tokens
		sixes       int
		roll        Roll
		wantEvents  []game.Event
		wantTurn    game.Seat
		wantPhase   Phase
		wantSixes   int
		wantMovable []Token
	}{
		{
			name: "no six in base passes the turn", blue: inBase(), roll: 5,
			wantEvents: []game.Event{RolledEvent{By: SeatBlue, Value: 5}, PassedEvent{By: SeatBlue}},
			wantTurn:   SeatYellow, wantPhase: PhaseRoll, wantMovable: []Token{},
		},
		{
			name: "six frees any base token", blue: inBase(), roll: 6,
			wantEvents: []game.Event{RolledEvent{By: SeatBlue, Value: 6}},
			wantTurn:   SeatBlue, wantPhase: PhaseMove, wantSixes: 1, wantMovable: []Token{1, 2, 3, 4},
		},
		{
			name: "six with no move still rolls again", blue: Tokens{52, 53, 54, home}, roll: 6,
			wantEvents: []game.Event{RolledEvent{By: SeatBlue, Value: 6}, PassedEvent{By: SeatBlue}},
			wantTurn:   SeatBlue, wantPhase: PhaseRoll, wantSixes: 1, wantMovable: []Token{},
		},
		{
			name: "third six forfeits the turn", blue: Tokens{10, 20, out, out}, sixes: 2, roll: 6,
			wantEvents: []game.Event{RolledEvent{By: SeatBlue, Value: 6}, ForfeitedEvent{By: SeatBlue}},
			wantTurn:   SeatYellow, wantPhase: PhaseRoll, wantMovable: []Token{},
		},
		{
			name: "a non-six breaks the run of sixes", blue: Tokens{10, out, out, out}, sixes: 2, roll: 3,
			wantEvents: []game.Event{RolledEvent{By: SeatBlue, Value: 3}},
			wantTurn:   SeatBlue, wantPhase: PhaseMove, wantMovable: []Token{1},
		},
		{
			name: "home needs an exact roll", blue: Tokens{53, 54, home, home}, roll: 3,
			wantEvents: []game.Event{RolledEvent{By: SeatBlue, Value: 3}},
			wantTurn:   SeatBlue, wantPhase: PhaseMove, wantMovable: []Token{1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := position(SeatBlue, 0, map[game.Seat]Tokens{SeatBlue: tt.blue, SeatYellow: inBase()})
			s.sixes = tt.sixes
			next, events := mustApply(t, s, Action{Kind: ActionRoll}, rngRolling(t, tt.roll))
			if diff := cmp.Diff(tt.wantEvents, events); diff != "" {
				t.Errorf("events (-want +got):\n%s", diff)
			}
			view := next.View(SeatYellow)
			got := []any{next.turn, view.Phase, view.SixesInRow, view.Movable, view.LastRoll}
			want := []any{tt.wantTurn, tt.wantPhase, tt.wantSixes, tt.wantMovable, tt.roll}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("turn, phase, sixes, movable, last roll (-want +got):\n%s", diff)
			}
		})
	}
}

func TestForfeitKeepsMovesMadeWithEarlierSixes(t *testing.T) {
	s := position(SeatBlue, 0, map[game.Seat]Tokens{SeatBlue: inBase(), SeatYellow: inBase()})
	for range 2 {
		s, _ = mustApply(t, s, Action{Kind: ActionRoll}, rngRolling(t, 6))
		s, _ = mustApply(t, s, Action{Kind: ActionMove, Token: 1}, nil)
	}
	s, _ = mustApply(t, s, Action{Kind: ActionRoll}, rngRolling(t, 6))
	if s.turn != SeatYellow || s.tokens[SeatBlue] != (Tokens{6, out, out, out}) {
		t.Errorf("after the third six: turn %s, blue %v; want yellow to roll and blue's token kept at 6", s.turn, s.tokens[SeatBlue])
	}
}

func TestMove(t *testing.T) {
	tests := []struct {
		name       string
		mover      game.Seat
		blue       Tokens
		yellow     Tokens
		roll       Roll
		token      Token
		wantBlue   Tokens
		wantYellow Tokens
		wantEvents []game.Event
		wantTurn   game.Seat
	}{
		{
			name: "six leaves base and rolls again", blue: inBase(), yellow: inBase(), roll: 6, token: 2,
			wantBlue: Tokens{out, 0, out, out}, wantYellow: inBase(), wantTurn: SeatBlue,
			wantEvents: []game.Event{MovedEvent{By: SeatBlue, Token: 2, From: out, To: 0}},
		},
		{
			name: "a plain move passes the turn", blue: Tokens{0, out, out, out}, yellow: inBase(), roll: 4, token: 1,
			wantBlue: Tokens{4, out, out, out}, wantYellow: inBase(), wantTurn: SeatYellow,
			wantEvents: []game.Event{MovedEvent{By: SeatBlue, Token: 1, From: 0, To: 4}},
		},
		{
			name: "landing on a lone token captures it without a bonus roll", blue: Tokens{2, out, out, out}, yellow: Tokens{out, 31, out, out}, roll: 3, token: 1,
			wantBlue: Tokens{5, out, out, out}, wantYellow: inBase(), wantTurn: SeatYellow,
			wantEvents: []game.Event{
				MovedEvent{By: SeatBlue, Token: 1, From: 2, To: 5},
				CapturedEvent{By: SeatBlue, Victim: SeatYellow, Token: 2},
			},
		},
		{
			name: "capture across the track wrap", mover: SeatYellow, blue: Tokens{2, out, out, out}, yellow: Tokens{25, out, out, out}, roll: 3, token: 1,
			wantBlue: inBase(), wantYellow: Tokens{28, out, out, out}, wantTurn: SeatBlue,
			wantEvents: []game.Event{
				MovedEvent{By: SeatYellow, Token: 1, From: 25, To: 28},
				CapturedEvent{By: SeatYellow, Victim: SeatBlue, Token: 1},
			},
		},
		{
			name: "a star square is safe", blue: Tokens{5, out, out, out}, yellow: Tokens{34, out, out, out}, roll: 3, token: 1,
			wantBlue: Tokens{8, out, out, out}, wantYellow: Tokens{34, out, out, out}, wantTurn: SeatYellow,
			wantEvents: []game.Event{MovedEvent{By: SeatBlue, Token: 1, From: 5, To: 8}},
		},
		{
			name: "a start square is safe", blue: Tokens{23, out, out, out}, yellow: Tokens{0, out, out, out}, roll: 3, token: 1,
			wantBlue: Tokens{26, out, out, out}, wantYellow: Tokens{0, out, out, out}, wantTurn: SeatYellow,
			wantEvents: []game.Event{MovedEvent{By: SeatBlue, Token: 1, From: 23, To: 26}},
		},
		{
			name: "an opponent pair on a safe square is shared", blue: Tokens{5, out, out, out}, yellow: Tokens{34, 34, out, out}, roll: 3, token: 1,
			wantBlue: Tokens{8, out, out, out}, wantYellow: Tokens{34, 34, out, out}, wantTurn: SeatYellow,
			wantEvents: []game.Event{MovedEvent{By: SeatBlue, Token: 1, From: 5, To: 8}},
		},
		{
			name: "own tokens stack", blue: Tokens{2, 5, out, out}, yellow: inBase(), roll: 3, token: 1,
			wantBlue: Tokens{5, 5, out, out}, wantYellow: inBase(), wantTurn: SeatYellow,
			wantEvents: []game.Event{MovedEvent{By: SeatBlue, Token: 1, From: 2, To: 5}},
		},
		{
			name: "a pair never blocks passing", blue: Tokens{2, out, out, out}, yellow: Tokens{31, 31, out, out}, roll: 5, token: 1,
			wantBlue: Tokens{7, out, out, out}, wantYellow: Tokens{31, 31, out, out}, wantTurn: SeatYellow,
			wantEvents: []game.Event{MovedEvent{By: SeatBlue, Token: 1, From: 2, To: 7}},
		},
		{
			name: "home columns are private", blue: Tokens{50, out, out, out}, yellow: Tokens{52, out, out, out}, roll: 2, token: 1,
			wantBlue: Tokens{52, out, out, out}, wantYellow: Tokens{52, out, out, out}, wantTurn: SeatYellow,
			wantEvents: []game.Event{MovedEvent{By: SeatBlue, Token: 1, From: 50, To: 52}},
		},
		{
			name: "reaching home with a six still rolls again", blue: Tokens{50, out, out, out}, yellow: inBase(), roll: 6, token: 1,
			wantBlue: Tokens{home, out, out, out}, wantYellow: inBase(), wantTurn: SeatBlue,
			wantEvents: []game.Event{MovedEvent{By: SeatBlue, Token: 1, From: 50, To: home}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mover := tt.mover
			if mover == "" {
				mover = SeatBlue
			}
			s := position(mover, tt.roll, map[game.Seat]Tokens{SeatBlue: tt.blue, SeatYellow: tt.yellow})
			next, events := mustApply(t, s, Action{Kind: ActionMove, Token: tt.token}, nil)
			if diff := cmp.Diff(tt.wantEvents, events); diff != "" {
				t.Errorf("events (-want +got):\n%s", diff)
			}
			wantProgress := map[game.Seat]Tokens{SeatBlue: tt.wantBlue, SeatYellow: tt.wantYellow}
			if diff := cmp.Diff(wantProgress, next.View(SeatBlue).Progress); diff != "" {
				t.Errorf("progress (-want +got):\n%s", diff)
			}
			if next.turn != tt.wantTurn || next.phase != PhaseRoll {
				t.Errorf("turn %s phase %s, want %s to roll", next.turn, next.phase, tt.wantTurn)
			}
		})
	}
}

func TestLastTokenHomeWins(t *testing.T) {
	s := position(SeatYellow, 3, map[game.Seat]Tokens{SeatBlue: inBase(), SeatYellow: {home, home, home, 53}})
	next, _ := mustApply(t, s, Action{Kind: ActionMove, Token: 4}, nil)
	result, finished := next.Outcome()
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatYellow}, Reason: ReasonAllHome}
	if diff := cmp.Diff(want, result); !finished || diff != "" {
		t.Errorf("finished=%t, result (-want +got):\n%s", finished, diff)
	}
	if view := next.View(SeatBlue); len(next.ToAct()) != 0 || view.Phase != PhaseFinished || len(view.Movable) != 0 {
		t.Errorf("after the win: ToAct %v, phase %s, movable %v; want none, finished, none", next.ToAct(), view.Phase, view.Movable)
	}
}

func TestIllegalActions(t *testing.T) {
	rolled := func(roll Roll, blue, yellow Tokens) state {
		return position(SeatBlue, roll, map[game.Seat]Tokens{SeatBlue: blue, SeatYellow: yellow})
	}
	tests := []struct {
		name    string
		state   state
		seat    game.Seat
		action  Action
		wantMsg string
	}{
		{"roll while a move is due", rolled(6, inBase(), inBase()), SeatBlue, Action{Kind: ActionRoll}, "move a token first"},
		{"move before rolling", rolled(0, inBase(), inBase()), SeatBlue, Action{Kind: ActionMove, Token: 1}, "roll first"},
		{"token out of range", rolled(6, inBase(), inBase()), SeatBlue, Action{Kind: ActionMove, Token: 5}, "token must be 1 to 4, got 5"},
		{"missing token", rolled(6, inBase(), inBase()), SeatBlue, Action{Kind: ActionMove}, "token must be 1 to 4, got 0"},
		{"leave base without a six", rolled(3, Tokens{out, 4, out, out}, inBase()), SeatBlue, Action{Kind: ActionMove, Token: 1}, "token 1 cannot move 3"},
		{"overshoot home", rolled(4, Tokens{53, 4, out, out}, inBase()), SeatBlue, Action{Kind: ActionMove, Token: 1}, "token 1 cannot move 4"},
		{"move a finished token", rolled(1, Tokens{home, 4, out, out}, inBase()), SeatBlue, Action{Kind: ActionMove, Token: 1}, "token 1 cannot move 1"},
		{"land on an opponent pair", rolled(3, Tokens{2, 4, out, out}, Tokens{31, 31, out, out}), SeatBlue, Action{Kind: ActionMove, Token: 1}, "token 1 cannot move 3"},
		{"unknown kind", rolled(0, inBase(), inBase()), SeatBlue, Action{Kind: "jump"}, "unknown action: jump"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tt.state.Apply(tt.seat, tt.action, nil)
			if err == nil || err.Error() != tt.wantMsg || !errors.As(err, new(game.IllegalActionError)) {
				t.Errorf("got %v, want illegal action %q", err, tt.wantMsg)
			}
		})
	}
	if _, _, err := rolled(0, inBase(), inBase()).Apply(SeatYellow, Action{Kind: ActionRoll}, nil); !errors.Is(err, game.ErrNotYourTurn) {
		t.Errorf("yellow rolling on blue's turn: got %v, want %v", err, game.ErrNotYourTurn)
	}
}

func TestOpponentPairIsNotMovable(t *testing.T) {
	s := position(SeatBlue, 3, map[game.Seat]Tokens{SeatBlue: {2, 4, out, out}, SeatYellow: {31, 31, out, out}})
	if got := s.View(SeatBlue).Movable; !slices.Equal(got, []Token{2}) {
		t.Errorf("movable = %v, want [2]", got)
	}
}

func TestEliminate(t *testing.T) {
	s := position(SeatRed, 4, map[game.Seat]Tokens{SeatBlue: {3, out, out, out}, SeatRed: {7, out, out, out}, SeatYellow: inBase()})
	st, events := s.Eliminate(SeatRed, game.ReasonTimeout)
	next := st.(state)
	if len(events) != 0 {
		t.Errorf("events = %v, want none", events)
	}
	if _, ok := next.View(SeatBlue).Progress[SeatRed]; ok {
		t.Error("red's tokens remain on the board")
	}
	if next.turn != SeatYellow || next.phase != PhaseRoll || len(next.View(SeatYellow).Movable) != 0 {
		t.Errorf("turn %s phase %s, want yellow to roll", next.turn, next.phase)
	}
	again, _ := next.Eliminate(SeatRed, game.ReasonTimeout)
	if diff := cmp.Diff(next.View(SeatBlue), again.View(SeatBlue)); diff != "" || !slices.Equal(again.ToAct(), next.ToAct()) {
		t.Errorf("eliminating red twice changed the game (-once +twice):\n%s", diff)
	}
	final, _ := next.Eliminate(SeatBlue, game.ReasonResign)
	result, finished := final.Outcome()
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatYellow}, Reason: game.ReasonResign}
	if phase := final.View(SeatYellow).Phase; phase != PhaseFinished {
		t.Errorf("phase %s after the last opponent resigns, want %s", phase, PhaseFinished)
	}
	if diff := cmp.Diff(want, result); !finished || diff != "" {
		t.Errorf("finished=%t, result (-want +got):\n%s", finished, diff)
	}
}

func snapshot(s state) state {
	s.order = slices.Clone(s.order)
	s.tokens = maps.Clone(s.tokens)
	s.result.Winners = slices.Clone(s.result.Winners)
	return s
}

func TestApplyAndEliminateLeaveTheEarlierStateUnchanged(t *testing.T) {
	board := map[game.Seat]Tokens{SeatBlue: {2, 50, out, out}, SeatYellow: {31, out, out, out}, SeatRed: {home, home, home, 55}}
	tests := []struct {
		name string
		s    state
		step func(state)
	}{
		{"six roll", position(SeatBlue, 0, board), func(s state) { mustApply(t, s, Action{Kind: ActionRoll}, rngRolling(t, 6)) }},
		{"capturing move", position(SeatBlue, 3, board), func(s state) { mustApply(t, s, Action{Kind: ActionMove, Token: 1}, nil) }},
		{"winning move", position(SeatRed, 1, board), func(s state) { mustApply(t, s, Action{Kind: ActionMove, Token: 4}, nil) }},
		{"eliminate", position(SeatBlue, 3, board), func(s state) { s.Eliminate(SeatBlue, game.ReasonResign) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := snapshot(tt.s)
			tt.step(tt.s)
			if diff := cmp.Diff(before, tt.s, cmp.AllowUnexported(state{})); diff != "" {
				t.Errorf("earlier state changed (-before +after):\n%s", diff)
			}
		})
	}
}

func TestEveryTimeControlStartsAGame(t *testing.T) {
	g := game.Adapt(Rules{})
	for _, value := range game.TimeControlOption().Values {
		t.Run(string(value), func(t *testing.T) {
			raw, err := json.Marshal(game.Options{game.OptionTimeControl: value})
			if err != nil {
				t.Fatal(err)
			}
			opts, err := g.ResolveOptions(raw)
			if err != nil {
				t.Fatal(err)
			}
			clock, err := opts.Clock()
			if err != nil || clock.Enabled != (value != game.OptionValue(game.TimeControlNone)) {
				t.Errorf("clock = %+v, %v", clock, err)
			}
			if _, seats, err := g.New(opts, "alice", []game.UserID{"bob"}, game.StepRNG(1, 0)); err != nil || len(seats) != 2 {
				t.Errorf("New: seats %v, %v", seats, err)
			}
		})
	}
}
