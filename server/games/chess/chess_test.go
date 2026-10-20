package chess

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"games.nvim/server/game"
	"games.nvim/server/game/gametest"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
)

func newGame(t *testing.T, color ColorChoice, seed uint64) (game.ErasedState, map[game.UserID]game.Seat) {
	t.Helper()
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(json.RawMessage(`{"color":"` + string(color) + `"}`))
	if err != nil {
		t.Fatal(err)
	}
	st, seats, err := g.New(opts, alice, []game.UserID{bob}, game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	return st, seats
}

func act(t *testing.T, st game.ErasedState, seat game.Seat, a Action) (game.ErasedState, error) {
	t.Helper()
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	next, _, err := st.Apply(seat, raw, game.StepRNG(1, 1))
	return next, err
}

func playSANs(t *testing.T, st game.ErasedState, line string) game.ErasedState {
	t.Helper()
	for san := range strings.FieldsSeq(line) {
		seat := st.ToAct()[0]
		var err error
		if st, err = act(t, st, seat, Action{Kind: ActionMove, Move: san}); err != nil {
			t.Fatalf("%s: %v", san, err)
		}
	}
	return st
}

func viewOf(t *testing.T, st game.ErasedState) View {
	t.Helper()
	raw, err := st.View(SeatWhite)
	if err != nil {
		t.Fatal(err)
	}
	var v View
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestAcceptsSANAndUCI(t *testing.T) {
	st, _ := newGame(t, ColorWhite, 1)
	st = playSANs(t, st, "e4 e7e5")
	want := "rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 2"
	if got := viewOf(t, st).FEN; got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestIllegalMoveMessage(t *testing.T) {
	st, _ := newGame(t, ColorWhite, 1)
	before := viewOf(t, st)
	_, err := act(t, st, SeatWhite, Action{Kind: ActionMove, Move: "Nf4"})
	var illegal game.IllegalActionError
	if !errors.As(err, &illegal) || illegal.Msg != "illegal move: Nf4" {
		t.Fatalf("got %v", err)
	}
	if after := viewOf(t, st); after.FEN != before.FEN {
		t.Errorf("state changed: %s", after.FEN)
	}
}

func TestNotYourTurn(t *testing.T) {
	st, _ := newGame(t, ColorWhite, 1)
	if _, err := act(t, st, SeatBlack, Action{Kind: ActionMove, Move: "e5"}); !errors.Is(err, game.ErrNotYourTurn) {
		t.Fatalf("got %v", err)
	}
}

func TestFoolsMate(t *testing.T) {
	st, _ := newGame(t, ColorWhite, 1)
	st = playSANs(t, st, "f3 e5 g4 Qh4#")
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatBlack}, Reason: ReasonCheckmate}
	got, ok := st.Outcome()
	if !ok || got.Kind != want.Kind || !slices.Equal(got.Winners, want.Winners) || got.Reason != want.Reason {
		t.Errorf("got %+v, %t", got, ok)
	}
	if v := viewOf(t, st); !v.InCheck || v.HistorySAN[len(v.HistorySAN)-1] != "Qh4#" {
		t.Errorf("view %+v", v)
	}
}

func TestStalemate(t *testing.T) {
	st, _ := newGame(t, ColorWhite, 1)
	st = playSANs(t, st, "e3 a5 Qh5 Ra6 Qxa5 h5 h4 Rah6 Qxc7 f6 Qxd7+ Kf7 Qxb7 Qd3 Qxb8 Qh7 Qxc8 Kg6 Qe6")
	got, ok := st.Outcome()
	if !ok || got.Kind != game.ResultDraw || got.Reason != ReasonStalemate {
		t.Errorf("got %+v, %t", got, ok)
	}
}

func TestClaimThreefold(t *testing.T) {
	st, _ := newGame(t, ColorWhite, 1)
	if _, err := act(t, st, SeatWhite, Action{Kind: ActionClaimDraw}); !errors.As(err, &game.IllegalActionError{}) {
		t.Fatalf("claim at start: got %v", err)
	}
	st = playSANs(t, st, "Nf3 Nf6 Ng1 Ng8 Nf3 Nf6 Ng1 Ng8")
	st, err := act(t, st, SeatWhite, Action{Kind: ActionClaimDraw})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := st.Outcome(); !ok || got.Kind != game.ResultDraw || got.Reason != ReasonThreefold {
		t.Errorf("got %+v, %t", got, ok)
	}
}

func TestColorOption(t *testing.T) {
	if _, seats := newGame(t, ColorWhite, 1); seats[alice] != SeatWhite {
		t.Errorf("white: %v", seats)
	}
	if _, seats := newGame(t, ColorBlack, 1); seats[alice] != SeatBlack {
		t.Errorf("black: %v", seats)
	}
	_, first := newGame(t, ColorRandom, 1)
	_, second := newGame(t, ColorRandom, 1)
	if first[alice] != second[alice] {
		t.Errorf("random not deterministic: %v vs %v", first, second)
	}
}

func moves(sans string) []gametest.Step[Action] {
	var steps []gametest.Step[Action]
	seat := SeatWhite
	for san := range strings.FieldsSeq(sans) {
		steps = append(steps, gametest.Step[Action]{Seat: seat, Action: Action{Kind: ActionMove, Move: san}})
		seat = map[game.Seat]game.Seat{SeatWhite: SeatBlack, SeatBlack: SeatWhite}[seat]
	}
	return steps
}

func TestConformance(t *testing.T) {
	base := gametest.Script[Opts, Action]{
		Opts:       Opts{TimeControl: TimeControlNone, Color: ColorWhite},
		Challenger: alice,
		Invitees:   []game.UserID{bob},
		Seed:       1,
	}
	script := func(name, line string, want game.Result, extra ...gametest.Step[Action]) gametest.Script[Opts, Action] {
		s := base
		s.Name, s.Steps, s.WantResult = name, append(moves(line), extra...), &want
		return s
	}
	repeated := "Nf3 Nf6 Ng1 Ng8 Nf3 Nf6 Ng1 Ng8"
	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		script("fools mate", "f3 e5 g4 Qh4#", game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatBlack}, Reason: ReasonCheckmate}),
		script("stalemate", "e3 a5 Qh5 Ra6 Qxa5 h5 h4 Rah6 Qxc7 f6 Qxd7+ Kf7 Qxb7 Qd3 Qxb8 Qh7 Qxc8 Kg6 Qe6", game.Result{Kind: game.ResultDraw, Reason: ReasonStalemate}),
		script("threefold claim", repeated, game.Result{Kind: game.ResultDraw, Reason: ReasonThreefold},
			gametest.Step[Action]{Seat: SeatWhite, Action: Action{Kind: ActionClaimDraw}}),
	})
}
