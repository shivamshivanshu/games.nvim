package wordle

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const solver game.UserID = "sam"

func newState(t *testing.T, seed game.Seed) state {
	t.Helper()
	st, seats, err := Rules{}.New(Opts{}, solver, nil, game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	if want := map[game.UserID]game.Seat{solver: SeatSolver}; !cmp.Equal(seats, want) {
		t.Fatalf("seats = %v, want %v", seats, want)
	}
	return st.(state)
}

func guess(text string) Action { return Action{Kind: ActionGuess, Text: text} }

// wrongGuesses returns n valid guesses that are not answer.
func wrongGuesses(answer string, n int) []string {
	words := slices.DeleteFunc([]string{"fight", "plumb", "crane", "shock", "gravy", "jumbo", "wordy"}, func(w string) bool { return w == answer })
	return words[:n]
}

func play(t *testing.T, st game.State[Action, View], texts ...string) game.State[Action, View] {
	t.Helper()
	for _, text := range texts {
		next, _, err := st.Apply(SeatSolver, guess(text), nil)
		if err != nil {
			t.Fatalf("guess %q: %v", text, err)
		}
		st = next
	}
	return st
}

func TestRejectedActions(t *testing.T) {
	st := newState(t, 1)
	tests := []struct {
		name   string
		seat   game.Seat
		action Action
		want   error
	}{
		{"too short", SeatSolver, guess("abc"), game.IllegalActionError{Msg: `a guess is 5 letters a-z, got "abc"`}},
		{"too long", SeatSolver, guess("cranes"), game.IllegalActionError{Msg: `a guess is 5 letters a-z, got "cranes"`}},
		{"empty", SeatSolver, guess(""), game.IllegalActionError{Msg: `a guess is 5 letters a-z, got ""`}},
		{"digit", SeatSolver, guess("cr4ne"), game.IllegalActionError{Msg: `a guess is 5 letters a-z, got "cr4ne"`}},
		{"non-ascii letter", SeatSolver, guess("crané"), game.IllegalActionError{Msg: `a guess is 5 letters a-z, got "crané"`}},
		{"not a word", SeatSolver, guess("ZZZZZ"), game.IllegalActionError{Msg: "not in word list: zzzzz"}},
		{"unknown kind", SeatSolver, Action{Kind: "set", Text: "crane"}, game.IllegalActionError{Msg: "unknown action: set"}},
		{"not a seat", "white", guess("crane"), game.ErrNotYourTurn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := st.Apply(tt.seat, tt.action, nil)
			if !errors.Is(err, tt.want) {
				t.Errorf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestGuessIgnoresCaseAndSurroundingSpace(t *testing.T) {
	st := newState(t, 1)
	next, events, err := st.Apply(SeatSolver, guess(" FiGhT "), nil)
	if err != nil {
		t.Fatal(err)
	}
	wantGuess := Guess{Word: "fight", Feedback: score("fight", st.answer)}
	if diff := cmp.Diff([]Guess{wantGuess}, next.View(SeatSolver).Guesses); diff != "" {
		t.Errorf("guesses (-want +got):\n%s", diff)
	}
	wantEvents := []game.Event{GuessedEvent{By: SeatSolver, Word: wantGuess.Word, Feedback: wantGuess.Feedback}}
	if diff := cmp.Diff(wantEvents, events); diff != "" {
		t.Errorf("events (-want +got):\n%s", diff)
	}
}

func TestViewHidesAnswerUntilFinished(t *testing.T) {
	st := newState(t, 1)
	misses := wrongGuesses(st.answer, MaxGuesses)
	tests := []struct {
		name          string
		state         game.State[Action, View]
		wantRemaining int
		wantAnswer    string
	}{
		{"start", st, MaxGuesses, ""},
		{"one guess left", play(t, st, misses[:MaxGuesses-1]...), 1, ""},
		{"out of guesses", play(t, st, misses...), 0, st.answer},
		{"solved", play(t, st, misses[0], st.answer), MaxGuesses - 2, st.answer},
		{"gave up", eliminated(st), MaxGuesses, st.answer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.View(SeatSolver)
			if got.Remaining != tt.wantRemaining || got.Answer != tt.wantAnswer || got.Guesses == nil {
				t.Errorf("view = %+v, want remaining %d, answer %q and non-nil guesses", got, tt.wantRemaining, tt.wantAnswer)
			}
		})
	}
}

func eliminated(st state) game.State[Action, View] {
	next, _ := st.Eliminate(SeatSolver, game.ReasonResign)
	return next
}

func TestApplyLeavesEarlierStatesUnchanged(t *testing.T) {
	st := newState(t, 1)
	misses := wrongGuesses(st.answer, 5)
	branchPoint := play(t, st, misses[:3]...)
	left := play(t, branchPoint, misses[3])
	play(t, branchPoint, misses[4])
	if got := words(branchPoint.View(SeatSolver)); !slices.Equal(got, misses[:3]) {
		t.Errorf("branch point now has %v, want %v", got, misses[:3])
	}
	if got, want := words(left.View(SeatSolver)), misses[:4]; !slices.Equal(got, want) {
		t.Errorf("sibling branch now has %v, want %v", got, want)
	}
}

func words(v View) []string {
	out := make([]string, len(v.Guesses))
	for i, g := range v.Guesses {
		out[i] = g.Word
	}
	return out
}

func TestNewRejectsOpponents(t *testing.T) {
	_, _, err := Rules{}.New(Opts{}, solver, []game.UserID{"bob"}, game.StepRNG(1, 0))
	if want := (game.BadRequestError{Msg: "wordle is solo, got 1 opponents"}); err != want {
		t.Errorf("got %v, want %v", err, want)
	}
}

func TestAnswerIsDrawnFromSeed(t *testing.T) {
	first, again, other := newState(t, 1).answer, newState(t, 1).answer, newState(t, 2).answer
	if first != again {
		t.Errorf("seed 1 drew %q then %q", first, again)
	}
	if first == other {
		t.Errorf("seeds 1 and 2 both drew %q", first)
	}
	for _, answer := range []string{first, other} {
		if !answerWords().contains(answer) {
			t.Errorf("answer %q is not from the answer list", answer)
		}
	}
}

func TestImposedEndingsRevealTheAnswerAndKeepAReachedResult(t *testing.T) {
	st := newState(t, 1)
	aborted := game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	solvedResult := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonSolved}
	solved := play(t, st, st.answer)
	tests := []struct {
		name  string
		state game.State[Action, View]
		want  game.Result
	}{
		{"abort before a guess", st.Conclude(aborted), aborted},
		{"abort after the solve", solved.(state).Conclude(aborted), solvedResult},
		{"give up after the solve", eliminated(solved.(state)), solvedResult},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, finished := tt.state.Outcome()
			if !finished || !cmp.Equal(got, tt.want) {
				t.Errorf("outcome = %+v, %t; want %+v", got, finished, tt.want)
			}
			if view := tt.state.View(SeatSolver); view.Answer != st.answer || len(tt.state.ToAct()) != 0 {
				t.Errorf("view %+v, to act %v; want the answer %q and nobody to act", view, tt.state.ToAct(), st.answer)
			}
		})
	}
}
