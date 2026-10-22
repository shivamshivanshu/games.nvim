package wordle

import (
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/feedback"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/games/words"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

const solver game.UserID = "sam"

func newState(t *testing.T, seed game.Seed) state {
	t.Helper()
	return gametest.NewSolo[state](t, Rules{}, Opts{}, solver, seed)
}

func guess(text string) Action { return Action{Kind: ActionGuess, Text: text} }

// wrongGuesses returns n valid guesses that are not answer.
func wrongGuesses(answer string, n int) []string {
	wrong := slices.DeleteFunc([]string{"fight", "plumb", "crane", "shock", "gravy", "jumbo", "wordy"}, func(w string) bool { return w == answer })
	return wrong[:n]
}

func play(t *testing.T, st game.State[Action, View], texts ...string) game.State[Action, View] {
	t.Helper()
	return gametest.PlaySolo(t, Rules{}.Info(), st, xslices.Map(texts, guess)...)
}

func TestRejectedActions(t *testing.T) {
	st := newState(t, 1)
	tests := []struct {
		name   string
		action Action
		want   error
	}{
		{"too short", guess("abc"), game.IllegalActionError{Msg: `a guess is 5 letters a-z, got "abc"`}},
		{"too long", guess("cranes"), game.IllegalActionError{Msg: `a guess is 5 letters a-z, got "cranes"`}},
		{"empty", guess(""), game.IllegalActionError{Msg: `a guess is 5 letters a-z, got ""`}},
		{"digit", guess("cr4ne"), game.IllegalActionError{Msg: `a guess is 5 letters a-z, got "cr4ne"`}},
		{"non-ascii letter", guess("crané"), game.IllegalActionError{Msg: `a guess is 5 letters a-z, got "crané"`}},
		{"not a word", guess("ZZZZZ"), game.IllegalActionError{Msg: "not in word list: zzzzz"}},
		{"unknown kind", Action{Kind: "set", Text: "crane"}, game.IllegalActionError{Msg: "unknown action: set"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := st.Apply(game.SeatSolo, tt.action, nil)
			if !errors.Is(err, tt.want) {
				t.Errorf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestGuessIgnoresCaseAndSurroundingSpace(t *testing.T) {
	st := newState(t, 1)
	next, events, err := st.Apply(game.SeatSolo, guess(" FiGhT "), nil)
	if err != nil {
		t.Fatal(err)
	}
	wantGuess := Guess{Word: "fight", Feedback: Feedback(feedback.Score("fight", st.answer))}
	if diff := cmp.Diff([]Guess{wantGuess}, next.View(game.SeatSolo).Guesses); diff != "" {
		t.Errorf("guesses (-want +got):\n%s", diff)
	}
	wantEvents := []game.Event{GuessedEvent{By: game.SeatSolo, Word: wantGuess.Word, Feedback: wantGuess.Feedback}}
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
		{"gave up", gametest.Resigned[Action, View](st), MaxGuesses, st.answer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.View(game.SeatSolo)
			if got.Remaining != tt.wantRemaining || got.Answer != tt.wantAnswer || got.Guesses == nil {
				t.Errorf("view = %+v, want remaining %d, answer %q and non-nil guesses", got, tt.wantRemaining, tt.wantAnswer)
			}
		})
	}
}

func TestApplyLeavesEarlierStatesUnchanged(t *testing.T) {
	st := newState(t, 1)
	misses := wrongGuesses(st.answer, 5)
	branchPoint := play(t, st, misses[:3]...)
	left := play(t, branchPoint, misses[3])
	play(t, branchPoint, misses[4])
	if got := guessedWords(branchPoint.View(game.SeatSolo)); !slices.Equal(got, misses[:3]) {
		t.Errorf("branch point now has %v, want %v", got, misses[:3])
	}
	if got, want := guessedWords(left.View(game.SeatSolo)), misses[:4]; !slices.Equal(got, want) {
		t.Errorf("sibling branch now has %v, want %v", got, want)
	}
}

func guessedWords(v View) []string {
	out := make([]string, len(v.Guesses))
	for i, g := range v.Guesses {
		out[i] = g.Word
	}
	return out
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
		if !words.Answers().Contains(answer) {
			t.Errorf("answer %q is not from the answer list", answer)
		}
	}
}

func TestImposedEndingsRevealTheAnswerAndKeepAReachedResult(t *testing.T) {
	st := newState(t, 1)
	aborted := game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	solvedResult := game.WinFor(ReasonSolved, game.SeatSolo)
	gaveUp := game.LossBy(game.ReasonResign)
	solved := play(t, st, st.answer)
	tests := []struct {
		name  string
		state game.State[Action, View]
		want  game.Result
	}{
		{"abort before a guess", st.Conclude(aborted), aborted},
		{"give up before solving", gametest.Resigned[Action, View](st), gaveUp},
		{"abort after the solve", solved.(state).Conclude(aborted), solvedResult},
		{"give up after the solve", gametest.Resigned(solved), solvedResult},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, finished := tt.state.Outcome()
			if !finished || !cmp.Equal(got, tt.want) {
				t.Errorf("outcome = %+v, %t; want %+v", got, finished, tt.want)
			}
			if view := tt.state.View(game.SeatSolo); view.Answer != st.answer || len(tt.state.ToAct()) != 0 {
				t.Errorf("view %+v, to act %v; want the answer %q and nobody to act", view, tt.state.ToAct(), st.answer)
			}
		})
	}
}

func TestViewKeyboardHoldsEveryGuessedLetter(t *testing.T) {
	st := newState(t, 1)
	misses := wrongGuesses(st.answer, 2)
	keyboard := play(t, st, misses...).View(game.SeatSolo).Keyboard
	letters := slices.Sorted(maps.Keys(keyboard))
	if want := []string{"b", "f", "g", "h", "i", "l", "m", "p", "t", "u"}; !slices.Equal(letters, want) {
		t.Errorf("keyboard after %v has letters %v, want %v", misses, letters, want)
	}
}

func TestEveryAnswerCanBeDrawn(t *testing.T) {
	const maxSeeds = 100_000
	answers := words.Answers()
	unseen := make(map[string]bool, answers.Len())
	for i := range answers.Len() {
		unseen[answers.At(i)] = true
	}
	for seed := game.Seed(0); seed < maxSeeds && len(unseen) > 0; seed++ {
		delete(unseen, newState(t, seed).answer)
	}
	if len(unseen) > 0 {
		t.Errorf("%d answers never drawn in %d seeds, e.g. %v", len(unseen), maxSeeds, slices.Sorted(maps.Keys(unseen))[:1])
	}
}
