package hangman

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/words"
)

const solver game.UserID = "sam"

func newState(t *testing.T, seed game.Seed) state {
	t.Helper()
	st, seats, err := Rules{}.New(Opts{}, solver, nil, game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	if want := map[game.UserID]game.Seat{solver: game.SeatSolo}; !cmp.Equal(seats, want) {
		t.Fatalf("seats = %v, want %v", seats, want)
	}
	return st.(state)
}

// seedWhere returns the first seed whose word satisfies keep.
func seedWhere(t *testing.T, keep func(word string) bool) game.Seed {
	t.Helper()
	for seed := range game.Seed(1000) {
		if keep(newState(t, seed).word) {
			return seed
		}
	}
	t.Fatal("no seed under 1000 draws such a word")
	return 0
}

func guess(text string) Action { return Action{Kind: ActionGuess, Text: text} }

// lettersOf returns the distinct letters of word, in order.
func lettersOf(word string) []string {
	var letters []string
	for _, letter := range strings.Split(word, "") {
		if !slices.Contains(letters, letter) {
			letters = append(letters, letter)
		}
	}
	return letters
}

// lettersNotIn returns the first n letters of the alphabet that word lacks.
func lettersNotIn(word string, n int) []string {
	return slices.DeleteFunc(strings.Split("abcdefghijklmnopqrstuvwxyz", ""), func(letter string) bool {
		return strings.Contains(word, letter)
	})[:n]
}

// wordsNot returns n listed words that are not word.
func wordsNot(word string, n int) []string {
	return slices.DeleteFunc([]string{"fight", "plumb", "crane", "shock", "gravy", "jumbo", "wordy"}, func(w string) bool {
		return w == word
	})[:n]
}

func play(t *testing.T, st game.State[Action, View], texts ...string) game.State[Action, View] {
	t.Helper()
	for _, text := range texts {
		next, _, err := st.Apply(game.SeatSolo, guess(text), nil)
		if err != nil {
			t.Fatalf("guess %q: %v", text, err)
		}
		st = next
	}
	return st
}

func TestRejectedActions(t *testing.T) {
	st := newState(t, 1)
	missed := lettersNotIn(st.word, 1)[0]
	wrongWord := wordsNot(st.word, 1)[0]
	played := play(t, st, missed, st.word[:1], wrongWord)
	shape := func(text string) error {
		return game.IllegalActionError{Msg: `a guess is one letter a-z or a 5-letter word, got "` + text + `"`}
	}
	tests := []struct {
		name   string
		action Action
		want   error
	}{
		{"empty", guess(""), shape("")},
		{"two letters", guess("ab"), shape("ab")},
		{"too long", guess("cranes"), shape("cranes")},
		{"digit", guess("4"), shape("4")},
		{"non-ascii letter", guess("é"), shape("é")},
		{"spaced letters", guess("a b"), shape("a b")},
		{"not a word", guess("ZZZZZ"), game.IllegalActionError{Msg: "not in word list: zzzzz"}},
		{"a missed letter again", guess(strings.ToUpper(missed)), game.IllegalActionError{Msg: "already guessed: " + missed}},
		{"a found letter again", guess(st.word[:1]), game.IllegalActionError{Msg: "already guessed: " + st.word[:1]}},
		{"a missed word again", guess(" " + wrongWord), game.IllegalActionError{Msg: "already guessed: " + wrongWord}},
		{"unknown kind", Action{Kind: "peek"}, game.IllegalActionError{Msg: "unknown action: peek"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := played.Apply(game.SeatSolo, tt.action, nil)
			if !errors.Is(err, tt.want) {
				t.Errorf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestALetterRevealsEveryCopyAndAMissIsCounted(t *testing.T) {
	st := newState(t, seedWhere(t, func(word string) bool { return len(lettersOf(word)) < len(word) }))
	repeated := ""
	for _, letter := range lettersOf(st.word) {
		if strings.Count(st.word, letter) > 1 {
			repeated = letter
		}
	}
	missed := lettersNotIn(st.word, 1)[0]
	next, events, err := st.Apply(game.SeatSolo, guess(strings.ToUpper(repeated)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []game.Event{GuessedEvent{By: game.SeatSolo, Guess: repeated, Hit: true}}; !cmp.Equal(events, want) {
		t.Errorf("events = %+v, want %+v", events, want)
	}
	next, events, err = next.Apply(game.SeatSolo, guess(missed), nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []game.Event{GuessedEvent{By: game.SeatSolo, Guess: missed, Hit: false}}; !cmp.Equal(events, want) {
		t.Errorf("events = %+v, want %+v", events, want)
	}
	wantPattern := make([]string, len(st.word))
	for i, letter := range strings.Split(st.word, "") {
		if letter == repeated {
			wantPattern[i] = letter
		}
	}
	want := View{
		Pattern:     wantPattern,
		Guessed:     []LetterGuess{{Letter: repeated, Hit: true}, {Letter: missed, Hit: false}},
		MissedWords: []string{},
		Misses:      1,
		Remaining:   MaxMisses - 1,
	}
	if got := next.View(game.SeatSolo); !cmp.Equal(got, want) {
		t.Errorf("view of %q = %+v, want %+v", st.word, got, want)
	}
}

func TestFindingEveryLetterSolves(t *testing.T) {
	st := newState(t, 2)
	letters := lettersOf(st.word)
	almost := play(t, st, append(lettersNotIn(st.word, MaxMisses-1), letters[:len(letters)-1]...)...)
	if _, finished := almost.Outcome(); finished || almost.View(game.SeatSolo).Word != "" {
		t.Fatalf("finished before the last letter: %+v", almost.View(game.SeatSolo))
	}
	end := play(t, almost, letters[len(letters)-1])
	assertSolved(t, end, st.word)
}

func TestTheWholeWordSolvesAndAWrongWordIsOneMiss(t *testing.T) {
	st := newState(t, 3)
	missedWords := wordsNot(st.word, 2)
	almost := play(t, st, append(missedWords, lettersNotIn(st.word, MaxMisses-3)...)...)
	if view := almost.View(game.SeatSolo); view.Misses != MaxMisses-1 || view.Remaining != 1 || !cmp.Equal(view.MissedWords, missedWords) {
		t.Fatalf("view = %+v, want %d misses, 1 remaining and missed words %v", view, MaxMisses-1, missedWords)
	}
	assertSolved(t, play(t, almost, strings.ToUpper(st.word)), st.word)
}

func assertSolved(t *testing.T, end game.State[Action, View], word string) {
	t.Helper()
	want := game.WinFor(ReasonSolved, game.SeatSolo)
	if got, finished := end.Outcome(); !finished || !cmp.Equal(got, want) {
		t.Errorf("outcome = %+v, %t; want %+v", got, finished, want)
	}
	if view := end.View(game.SeatSolo); view.Word != word || strings.Join(view.Pattern, "") != word {
		t.Errorf("view = %+v, want the word %q revealed in full", view, word)
	}
}

func TestSixMissesHang(t *testing.T) {
	st := newState(t, 4)
	misses := append(wordsNot(st.word, 1), lettersNotIn(st.word, MaxMisses-1)...)
	almost := play(t, st, append([]string{st.word[:1]}, misses[:MaxMisses-1]...)...)
	if _, finished := almost.Outcome(); finished {
		t.Fatalf("hanged after %d misses", MaxMisses-1)
	}
	end := play(t, almost, misses[MaxMisses-1])
	want := game.LossBy(ReasonHanged)
	if got, finished := end.Outcome(); !finished || !cmp.Equal(got, want) {
		t.Errorf("outcome = %+v, %t; want %+v", got, finished, want)
	}
	view := end.View(game.SeatSolo)
	if view.Word != st.word || view.Misses != MaxMisses || view.Remaining != 0 || strings.Join(view.Pattern, "") == st.word {
		t.Errorf("view = %+v; want the word %q, %d misses, none left, and only the found letters in the pattern", view, st.word, MaxMisses)
	}
}

func TestApplyLeavesTheEarlierStateUnchanged(t *testing.T) {
	st := newState(t, 5)
	first := lettersNotIn(st.word, 2)
	branchPoint := play(t, st, first[0], wordsNot(st.word, 1)[0])
	before := branchPoint.View(game.SeatSolo)
	play(t, branchPoint, first[1], wordsNot(st.word, 2)[1])
	if after := branchPoint.View(game.SeatSolo); !cmp.Equal(after, before) {
		t.Errorf("branch point changed from %+v to %+v", before, after)
	}
}

func TestWordIsDrawnFromSeedAndTheAnswerList(t *testing.T) {
	first, again, other := newState(t, 1).word, newState(t, 1).word, newState(t, 2).word
	if first != again {
		t.Errorf("seed 1 drew %q then %q", first, again)
	}
	if first == other {
		t.Errorf("seeds 1 and 2 both drew %q", first)
	}
	for _, word := range []string{first, other} {
		if !words.Answers().Contains(word) {
			t.Errorf("word %q is not from the answer list", word)
		}
	}
}

func eliminated(st game.State[Action, View]) game.State[Action, View] {
	next, _ := st.Eliminate(game.SeatSolo, game.ReasonResign)
	return next
}

func TestImposedEndingsRevealTheWordAndKeepAReachedResult(t *testing.T) {
	st := newState(t, 1)
	aborted := game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	solvedResult := game.WinFor(ReasonSolved, game.SeatSolo)
	gaveUp := game.LossBy(game.ReasonResign)
	solved := play(t, st, st.word)
	tests := []struct {
		name  string
		state game.State[Action, View]
		want  game.Result
	}{
		{"abort before a guess", st.Conclude(aborted), aborted},
		{"give up before solving", eliminated(st), gaveUp},
		{"abort after the solve", solved.(state).Conclude(aborted), solvedResult},
		{"give up after the solve", eliminated(solved), solvedResult},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, finished := tt.state.Outcome()
			if !finished || !cmp.Equal(got, tt.want) {
				t.Errorf("outcome = %+v, %t; want %+v", got, finished, tt.want)
			}
			if view := tt.state.View(game.SeatSolo); view.Word != st.word || len(tt.state.ToAct()) != 0 {
				t.Errorf("view %+v, to act %v; want the word %q and nobody to act", view, tt.state.ToAct(), st.word)
			}
		})
	}
}

func TestCatalogueInputPatternMatchesTheGuessesParsed(t *testing.T) {
	pattern := regexp.MustCompile(Rules{}.Info().Actions[0].Input.Pattern)
	inputs := []string{
		"e", " E ", "crane", " CRANE\t", "ab", "cranes", "", "4", "é", "c rane",
		"K", " e", "e　", "zzzzz",
	}
	for _, text := range inputs {
		_, err := parseGuess(text)
		shaped := err == nil || strings.HasPrefix(err.Error(), "not in word list")
		if got := pattern.MatchString(text); got != shaped {
			t.Errorf("pattern matches %q: %t, but the server parses it as a guess: %t", text, got, shaped)
		}
	}
}
