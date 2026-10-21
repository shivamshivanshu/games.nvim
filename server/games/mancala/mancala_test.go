package mancala

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
)

func sow(pit string) Action { return Action{Kind: ActionSow, Pit: pit} }

// at builds a board from each side's pits, pit 1 first, and its store.
func at(south [pitsPerSide]int, southStore int, north [pitsPerSide]int, northStore int) board {
	var b board
	copy(b[firstHole(SeatSouth):], south[:])
	copy(b[firstHole(SeatNorth):], north[:])
	b[storeHole(SeatSouth)], b[storeHole(SeatNorth)] = southStore, northStore
	return b
}

func newState(t *testing.T) state {
	t.Helper()
	st, _, err := Rules{}.New(Opts{First: game.FirstMoverMe}, alice, []game.UserID{bob}, game.StepRNG(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	return st.(state)
}

func sowAll(t *testing.T, s state, pits ...string) (state, []game.Event) {
	t.Helper()
	var events []game.Event
	for _, pit := range pits {
		s, events = gametest.MustApply(t, s, s.toMove, sow(pit), nil)
	}
	return s, events
}

func TestNewSeatsTheFirstMoverSouth(t *testing.T) {
	cases := []struct {
		first     game.FirstMover
		seed      game.Seed
		wantSouth game.UserID
	}{
		{game.FirstMoverMe, 1, alice},
		{game.FirstMoverOpponent, 1, bob},
		{game.FirstMoverRandom, 1, bob},
		{game.FirstMoverRandom, 6, alice},
	}
	for _, tc := range cases {
		t.Run(string(tc.first), func(t *testing.T) {
			st, seats, err := Rules{}.New(Opts{First: tc.first}, alice, []game.UserID{bob}, game.StepRNG(tc.seed, 0))
			if err != nil {
				t.Fatal(err)
			}
			if seats[tc.wantSouth] != SeatSouth || len(seats) != 2 {
				t.Errorf("seats %v, want %s south", seats, tc.wantSouth)
			}
			if diff := cmp.Diff([]game.Seat{SeatSouth}, st.ToAct()); diff != "" {
				t.Errorf("to act (-want +got):\n%s", diff)
			}
		})
	}
}

func TestStartView(t *testing.T) {
	s := newState(t)
	four := []int{4, 4, 4, 4, 4, 4}
	want := View{South: Side{Pits: four, Store: 0}, North: Side{Pits: four, Store: 0}, LegalPits: []int{1, 2, 3, 4, 5, 6}}
	if diff := cmp.Diff(want, s.View(SeatSouth)); diff != "" {
		t.Errorf("south's view (-want +got):\n%s", diff)
	}
	if got := s.View(SeatNorth).LegalPits; got == nil || len(got) != 0 {
		t.Errorf("north's legal pits %v, want [] while south moves", got)
	}
}

func TestSowing(t *testing.T) {
	cases := []struct {
		name      string
		from      board
		mover     game.Seat
		pit       string
		want      board
		wantEvent SowedEvent
		wantNext  game.Seat
	}{
		{
			name:      "one seed per pit counter-clockwise",
			from:      startBoard(),
			mover:     SeatSouth,
			pit:       "1",
			want:      at([6]int{0, 5, 5, 5, 5, 4}, 0, [6]int{4, 4, 4, 4, 4, 4}, 0),
			wantEvent: SowedEvent{By: SeatSouth, Pit: 1},
			wantNext:  SeatNorth,
		},
		{
			name:      "the last seed in the mover's store moves again",
			from:      startBoard(),
			mover:     SeatSouth,
			pit:       "3",
			want:      at([6]int{4, 4, 0, 5, 5, 5}, 1, [6]int{4, 4, 4, 4, 4, 4}, 0),
			wantEvent: SowedEvent{By: SeatSouth, Pit: 3, Again: true},
			wantNext:  SeatSouth,
		},
		{
			name:      "north sows from its own pit 1 toward its store",
			from:      startBoard(),
			mover:     SeatNorth,
			pit:       "3",
			want:      at([6]int{4, 4, 4, 4, 4, 4}, 0, [6]int{4, 4, 0, 5, 5, 5}, 1),
			wantEvent: SowedEvent{By: SeatNorth, Pit: 3, Again: true},
			wantNext:  SeatNorth,
		},
		{
			name:      "south skips north's store",
			from:      at([6]int{1, 1, 1, 1, 1, 10}, 0, [6]int{1, 1, 1, 1, 1, 1}, 0),
			mover:     SeatSouth,
			pit:       "6",
			want:      at([6]int{2, 2, 2, 1, 1, 0}, 1, [6]int{2, 2, 2, 2, 2, 2}, 0),
			wantEvent: SowedEvent{By: SeatSouth, Pit: 6},
			wantNext:  SeatNorth,
		},
		{
			name:      "north skips south's store",
			from:      at([6]int{1, 1, 1, 1, 1, 1}, 0, [6]int{1, 1, 1, 1, 1, 8}, 0),
			mover:     SeatNorth,
			pit:       "6",
			want:      at([6]int{2, 2, 2, 2, 2, 2}, 0, [6]int{2, 1, 1, 1, 1, 0}, 1),
			wantEvent: SowedEvent{By: SeatNorth, Pit: 6},
			wantNext:  SeatSouth,
		},
		{
			name:      "an empty pit of the mover's captures itself and the pit facing it",
			from:      at([6]int{1, 0, 1, 1, 1, 1}, 0, [6]int{1, 1, 1, 1, 7, 1}, 0),
			mover:     SeatSouth,
			pit:       "1",
			want:      at([6]int{0, 0, 1, 1, 1, 1}, 8, [6]int{1, 1, 1, 1, 0, 1}, 0),
			wantEvent: SowedEvent{By: SeatSouth, Pit: 1, Captured: 8},
			wantNext:  SeatNorth,
		},
		{
			name:      "north captures from south",
			from:      at([6]int{1, 1, 1, 3, 1, 1}, 0, [6]int{0, 1, 0, 2, 1, 1}, 0),
			mover:     SeatNorth,
			pit:       "2",
			want:      at([6]int{1, 1, 1, 0, 1, 1}, 0, [6]int{0, 0, 0, 2, 1, 1}, 4),
			wantEvent: SowedEvent{By: SeatNorth, Pit: 2, Captured: 4},
			wantNext:  SeatSouth,
		},
		{
			name:      "no capture when the facing pit is empty",
			from:      at([6]int{1, 0, 1, 1, 1, 1}, 0, [6]int{1, 1, 1, 1, 0, 1}, 0),
			mover:     SeatSouth,
			pit:       "1",
			want:      at([6]int{0, 1, 1, 1, 1, 1}, 0, [6]int{1, 1, 1, 1, 0, 1}, 0),
			wantEvent: SowedEvent{By: SeatSouth, Pit: 1},
			wantNext:  SeatNorth,
		},
		{
			name:      "no capture in the opponent's empty pit",
			from:      at([6]int{1, 1, 1, 1, 1, 2}, 0, [6]int{0, 1, 1, 1, 1, 1}, 0),
			mover:     SeatSouth,
			pit:       "6",
			want:      at([6]int{1, 1, 1, 1, 1, 0}, 1, [6]int{1, 1, 1, 1, 1, 1}, 0),
			wantEvent: SowedEvent{By: SeatSouth, Pit: 6},
			wantNext:  SeatNorth,
		},
		{
			name:      "no capture in a pit that held seeds",
			from:      at([6]int{1, 1, 1, 1, 1, 1}, 0, [6]int{1, 1, 1, 1, 5, 1}, 0),
			mover:     SeatSouth,
			pit:       "1",
			want:      at([6]int{0, 2, 1, 1, 1, 1}, 0, [6]int{1, 1, 1, 1, 5, 1}, 0),
			wantEvent: SowedEvent{By: SeatSouth, Pit: 1},
			wantNext:  SeatNorth,
		},
		{
			name:      "thirteen seeds lap into the emptied pit and capture",
			from:      at([6]int{13, 1, 1, 1, 1, 1}, 0, [6]int{1, 1, 1, 1, 1, 2}, 0),
			mover:     SeatSouth,
			pit:       "1",
			want:      at([6]int{0, 2, 2, 2, 2, 2}, 1+1+3, [6]int{2, 2, 2, 2, 2, 0}, 0),
			wantEvent: SowedEvent{By: SeatSouth, Pit: 1, Captured: 4},
			wantNext:  SeatNorth,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := state{board: tc.from, toMove: tc.mover}
			next, events, err := s.Apply(tc.mover, sow(tc.pit), nil)
			if err != nil {
				t.Fatal(err)
			}
			got := next.(state)
			if diff := cmp.Diff(tc.want, got.board); diff != "" {
				t.Errorf("board (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff([]game.Event{tc.wantEvent}, events); diff != "" {
				t.Errorf("events (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff([]game.Seat{tc.wantNext}, got.ToAct()); diff != "" {
				t.Errorf("to act (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTheGameEndsWhenOneSideIsEmpty(t *testing.T) {
	cases := []struct {
		name       string
		from       board
		mover      game.Seat
		pit        string
		want       board
		wantEvents []game.Event
		wantResult game.Result
	}{
		{
			name:  "the mover empties its own side and the opponent collects",
			from:  at([6]int{0, 0, 0, 0, 0, 1}, 20, [6]int{1, 2, 3, 0, 0, 0}, 21),
			mover: SeatSouth,
			pit:   "6",
			want:  at([6]int{}, 21, [6]int{}, 27),
			wantEvents: []game.Event{
				SowedEvent{By: SeatSouth, Pit: 6},
				CollectedEvent{By: SeatNorth, Seeds: 6},
			},
			wantResult: game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatNorth}, Reason: ReasonMoreSeeds},
		},
		{
			name:  "a capture empties the opponent's side and the mover collects",
			from:  at([6]int{1, 0, 2, 0, 0, 0}, 22, [6]int{0, 0, 0, 0, 3, 0}, 20),
			mover: SeatSouth,
			pit:   "1",
			want:  at([6]int{}, 28, [6]int{}, 20),
			wantEvents: []game.Event{
				SowedEvent{By: SeatSouth, Pit: 1, Captured: 4},
				CollectedEvent{By: SeatSouth, Seeds: 2},
			},
			wantResult: game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSouth}, Reason: ReasonMoreSeeds},
		},
		{
			name:  "the last seed in the store does not move again once the game is over",
			from:  at([6]int{0, 0, 0, 0, 0, 1}, 25, [6]int{1, 1, 0, 0, 0, 0}, 20),
			mover: SeatSouth,
			pit:   "6",
			want:  at([6]int{}, 26, [6]int{}, 22),
			wantEvents: []game.Event{
				SowedEvent{By: SeatSouth, Pit: 6},
				CollectedEvent{By: SeatNorth, Seeds: 2},
			},
			wantResult: game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSouth}, Reason: ReasonMoreSeeds},
		},
		{
			name:  "equal stores draw",
			from:  at([6]int{0, 0, 0, 0, 0, 1}, 23, [6]int{0, 0, 0, 0, 0, 2}, 22),
			mover: SeatSouth,
			pit:   "6",
			want:  at([6]int{}, 24, [6]int{}, 24),
			wantEvents: []game.Event{
				SowedEvent{By: SeatSouth, Pit: 6},
				CollectedEvent{By: SeatNorth, Seeds: 2},
			},
			wantResult: game.Result{Kind: game.ResultDraw, Reason: ReasonEqualSeeds},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := state{board: tc.from, toMove: tc.mover}
			next, events, err := s.Apply(tc.mover, sow(tc.pit), nil)
			if err != nil {
				t.Fatal(err)
			}
			got := next.(state)
			if diff := cmp.Diff(tc.want, got.board); diff != "" {
				t.Errorf("board (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantEvents, events); diff != "" {
				t.Errorf("events (-want +got):\n%s", diff)
			}
			result, finished := got.Outcome()
			if diff := cmp.Diff(tc.wantResult, result); !finished || diff != "" {
				t.Errorf("finished %t, result (-want +got):\n%s", finished, diff)
			}
			for _, seat := range []game.Seat{SeatSouth, SeatNorth} {
				if legal := got.View(seat).LegalPits; len(legal) != 0 {
					t.Errorf("%s's legal pits %v after the end", seat, legal)
				}
			}
		})
	}
}

func TestRejectedActions(t *testing.T) {
	afterPitThree, _ := sowAll(t, newState(t), "3")
	cases := []struct {
		name    string
		state   state
		seat    game.Seat
		action  Action
		wantErr error
	}{
		{"an empty pit", afterPitThree, SeatSouth, sow("3"), game.IllegalActionError{Msg: "pit 3 is empty"}},
		{"pit below range", newState(t), SeatSouth, sow("0"), game.IllegalActionError{Msg: `pit must be 1 to 6, got "0"`}},
		{"pit above range", newState(t), SeatSouth, sow("7"), game.IllegalActionError{Msg: `pit must be 1 to 6, got "7"`}},
		{"pit with spaces, which the catalogue does not list", newState(t), SeatSouth, sow(" 1"), game.IllegalActionError{Msg: `pit must be 1 to 6, got " 1"`}},
		{"pit missing", newState(t), SeatSouth, sow(""), game.IllegalActionError{Msg: `pit must be 1 to 6, got ""`}},
		{"unknown kind", newState(t), SeatSouth, Action{Kind: "drop", Pit: "1"}, game.IllegalActionError{Msg: "unknown action: drop"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tc.state.Apply(tc.seat, tc.action, nil)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestLegalPitsAreTheMoversNonEmptyPits(t *testing.T) {
	s := state{board: at([6]int{0, 3, 0, 1, 0, 0}, 0, [6]int{2, 2, 2, 2, 2, 2}, 0), toMove: SeatSouth}
	if diff := cmp.Diff([]int{2, 4}, s.View(SeatSouth).LegalPits); diff != "" {
		t.Errorf("south's legal pits (-want +got):\n%s", diff)
	}
	if got := s.View(SeatNorth).LegalPits; len(got) != 0 {
		t.Errorf("north's legal pits %v while south moves", got)
	}
}

func TestConcludeStopsOfferingPitsAndKeepsAReachedResult(t *testing.T) {
	agreed := game.Result{Kind: game.ResultDraw, Reason: game.ReasonDrawAgreed}
	concluded := newState(t).Conclude(agreed)
	if got, finished := concluded.Outcome(); !finished || !cmp.Equal(got, agreed) {
		t.Errorf("outcome %+v %t, want %+v", got, finished, agreed)
	}
	if legal := concluded.View(SeatSouth).LegalPits; len(legal) != 0 {
		t.Errorf("legal pits %v after a draw was agreed", legal)
	}
	played := game.Result{Kind: game.ResultDraw, Reason: ReasonEqualSeeds}
	ended := state{Ending: game.Ending{}.Concluded(played), board: at([6]int{}, 24, [6]int{}, 24)}
	if got, _ := ended.Conclude(agreed).Outcome(); !cmp.Equal(got, played) {
		t.Errorf("Conclude replaced a played result with %+v", got)
	}
}

func TestSeedsAreConservedThroughAPlayedOutGame(t *testing.T) {
	_, final := playedOut(t)
	total := 0
	for _, seeds := range final.board {
		total += seeds
	}
	if total != 2*pitsPerSide*startSeeds {
		t.Errorf("%d seeds at the end, want %d", total, 2*pitsPerSide*startSeeds)
	}
	if !final.Over() || !final.board.pitsEmpty(SeatSouth) || !final.board.pitsEmpty(SeatNorth) {
		t.Errorf("played-out game: finished %t, board %v; want finished with every pit empty", final.Over(), final.board)
	}
}

func TestViewWire(t *testing.T) {
	raw, err := json.Marshal(newState(t).View(SeatNorth))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"south":{"pits":[4,4,4,4,4,4],"store":0},"north":{"pits":[4,4,4,4,4,4],"store":0},"legal_pits":[]}`
	if string(raw) != want {
		t.Errorf("view\n got %s\nwant %s", raw, want)
	}
}
