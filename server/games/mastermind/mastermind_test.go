package mastermind

import (
	"errors"
	"regexp"
	"slices"
	"strings"
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

func guess(code string) Action { return Action{Kind: ActionGuess, Code: code} }

// wrongCodes returns n distinct codes that are not secret.
func wrongCodes(secret Code, n int) []string {
	codes := slices.DeleteFunc(
		[]string{"rrrr", "gggg", "bbbb", "yyyy", "oooo", "pppp", "rgby", "opop", "bygr", "yrog", "gbpo"},
		func(c string) bool { return Code(c) == secret },
	)
	return codes[:n]
}

func play(t *testing.T, st game.State[Action, View], codes ...string) game.State[Action, View] {
	t.Helper()
	for _, code := range codes {
		next, _, err := st.Apply(SeatSolver, guess(code), nil)
		if err != nil {
			t.Fatalf("guess %q: %v", code, err)
		}
		st = next
	}
	return st
}

func TestGrade(t *testing.T) {
	tests := []struct {
		guess, secret Code
		black, white  int
	}{
		{"rgby", "rgby", 4, 0},
		{"oooo", "rgby", 0, 0},
		{"ybgr", "rgby", 0, 4},
		{"rgyb", "rgby", 2, 2},
		{"rrgg", "rgby", 1, 1},
		{"rrrr", "rrgb", 2, 0},
		{"rrgb", "rrrr", 2, 0},
		{"brrr", "rrbb", 1, 2},
		{"gggr", "rggg", 2, 2},
	}
	for _, tt := range tests {
		got := grade(tt.guess, tt.secret)
		if want := (Guess{Code: tt.guess, Black: tt.black, White: tt.white}); got != want {
			t.Errorf("grade(%q, %q) = %+v, want %+v", tt.guess, tt.secret, got, want)
		}
	}
}

func TestRejectedActions(t *testing.T) {
	st := newState(t, 1)
	tests := []struct {
		name   string
		action Action
		want   error
	}{
		{"too short", guess("rgb"), game.IllegalActionError{Msg: `a guess is 4 colours from rgbyop, got "rgb"`}},
		{"too long", guess("rgbyo"), game.IllegalActionError{Msg: `a guess is 4 colours from rgbyop, got "rgbyo"`}},
		{"empty", guess(""), game.IllegalActionError{Msg: `a guess is 4 colours from rgbyop, got ""`}},
		{"not a colour", guess("rgbw"), game.IllegalActionError{Msg: `a guess is 4 colours from rgbyop, got "rgbw"`}},
		{"spaces between pegs", guess("r g b y"), game.IllegalActionError{Msg: `a guess is 4 colours from rgbyop, got "r g b y"`}},
		{"unknown kind", Action{Kind: "peek"}, game.IllegalActionError{Msg: "unknown action: peek"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := st.Apply(SeatSolver, tt.action, nil)
			if !errors.Is(err, tt.want) {
				t.Errorf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestGuessIgnoresCaseAndSurroundingSpace(t *testing.T) {
	st := newState(t, 1)
	code := wrongCodes(st.secret, 1)[0]
	next, events, err := st.Apply(SeatSolver, guess(" "+strings.ToUpper(code)+"\t"), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := grade(Code(code), st.secret)
	if got := next.View(SeatSolver).Guesses; !cmp.Equal(got, []Guess{want}) {
		t.Errorf("guesses = %+v, want %+v", got, []Guess{want})
	}
	wantEvents := []game.Event{GuessedEvent{By: SeatSolver, Code: want.Code, Black: want.Black, White: want.White}}
	if !cmp.Equal(events, wantEvents) {
		t.Errorf("events = %+v, want %+v", events, wantEvents)
	}
	if len(st.guesses) != 0 {
		t.Errorf("Apply changed the earlier state: %+v", st.guesses)
	}
}

func TestSolvedOnTheLastGuessWins(t *testing.T) {
	st := newState(t, 2)
	end := play(t, st, append(wrongCodes(st.secret, MaxGuesses-1), string(st.secret))...)
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonSolved}
	if got, finished := end.Outcome(); !finished || !cmp.Equal(got, want) {
		t.Errorf("outcome = %+v, %t; want %+v", got, finished, want)
	}
	if view := end.View(SeatSolver); view.Secret != st.secret || view.Remaining != 0 {
		t.Errorf("view secret %q, remaining %d; want %q and 0", view.Secret, view.Remaining, st.secret)
	}
}

func TestTenWrongGuessesLose(t *testing.T) {
	st := newState(t, 3)
	almost := play(t, st, wrongCodes(st.secret, MaxGuesses-1)...)
	if _, finished := almost.Outcome(); finished || almost.View(SeatSolver).Secret != "" || almost.View(SeatSolver).Remaining != 1 {
		t.Fatalf("after %d guesses: finished %t, view %+v; want one guess left and the secret hidden", MaxGuesses-1, finished, almost.View(SeatSolver))
	}
	end := play(t, almost, wrongCodes(st.secret, MaxGuesses)[MaxGuesses-1])
	want := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{}, Reason: ReasonOutOfGuesses}
	if got, finished := end.Outcome(); !finished || !cmp.Equal(got, want) {
		t.Errorf("outcome = %+v, %t; want %+v", got, finished, want)
	}
	if got := end.View(SeatSolver).Secret; got != st.secret {
		t.Errorf("secret = %q, want %q", got, st.secret)
	}
}

func TestViewDescribesTheBoard(t *testing.T) {
	view := newState(t, 1).View(SeatSolver)
	want := View{Palette: []string{"r", "g", "b", "y", "o", "p"}, CodeLength: 4, Guesses: []Guess{}, Remaining: 10}
	if !cmp.Equal(view, want) {
		t.Errorf("view = %+v, want %+v", view, want)
	}
}

func TestSecretIsDrawnFromSeedWithEveryColourAndRepeats(t *testing.T) {
	if first, again := newState(t, 1).secret, newState(t, 1).secret; first != again {
		t.Errorf("seed 1 drew %q then %q", first, again)
	}
	seen := map[rune]bool{}
	repeated := false
	for seed := range game.Seed(200) {
		secret := newState(t, seed).secret
		if !codeTextPattern.MatchString(string(secret)) {
			t.Fatalf("seed %d drew %q, not a code", seed, secret)
		}
		for i, peg := range string(secret) {
			seen[peg] = true
			repeated = repeated || strings.ContainsRune(string(secret[i+1:]), peg)
		}
	}
	if len(seen) != len(Palette) || !repeated {
		t.Errorf("200 seeds drew colours %v (repeats: %t); want all of %s and some repeat", seen, repeated, Palette)
	}
}

func eliminated(st game.State[Action, View]) game.State[Action, View] {
	next, _ := st.Eliminate(SeatSolver, game.ReasonResign)
	return next
}

func TestImposedEndingsRevealTheSecretAndKeepAReachedResult(t *testing.T) {
	st := newState(t, 1)
	aborted := game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}
	solvedResult := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonSolved}
	gaveUp := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{}, Reason: game.ReasonResign}
	solved := play(t, st, string(st.secret))
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
			if view := tt.state.View(SeatSolver); view.Secret != st.secret || len(tt.state.ToAct()) != 0 {
				t.Errorf("view %+v, to act %v; want the secret %q and nobody to act", view, tt.state.ToAct(), st.secret)
			}
		})
	}
}

func TestCatalogueInputPatternMatchesTheCodesParsed(t *testing.T) {
	pattern := regexp.MustCompile(Rules{}.Info().Actions[0].Input.Pattern)
	inputs := []string{"rgby", " RGBY ", "Oppo\t", "rgbyo", "rgb", "rgbw", "", "r gb", " rgby", "rgby　", "rgbK"}
	for _, text := range inputs {
		_, err := parseCode(text)
		if got := pattern.MatchString(text); got != (err == nil) {
			t.Errorf("pattern matches %q: %t, but the server parses it: %t", text, got, err == nil)
		}
	}
}
