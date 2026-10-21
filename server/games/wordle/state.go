package wordle

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/feedback"
	"github.com/shivamshivanshu/games.nvim/server/games/words"
)

type state struct {
	answer   string
	guesses  []Guess
	result   game.Result
	finished bool
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if !slices.Contains(s.ToAct(), seat) {
		return nil, nil, game.ErrNotYourTurn
	}
	if action.Kind != ActionGuess {
		return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
	}
	word, err := parseGuess(action.Text)
	if err != nil {
		return nil, nil, err
	}
	latest := Guess{Word: word, Feedback: Feedback(feedback.Score(word, s.answer))}
	return s.with(latest), []game.Event{GuessedEvent{By: seat, Word: latest.Word, Feedback: latest.Feedback}}, nil
}

// patternSpace is the whitespace \s matches in the catalogue's RE2 pattern: ASCII only.
const patternSpace = " \t\n\f\r"

func parseGuess(text string) (string, error) {
	trimmed := strings.Trim(text, patternSpace)
	if len(trimmed) != WordLength || strings.ContainsFunc(trimmed, isNotASCIILetter) {
		return "", game.IllegalActionError{Msg: fmt.Sprintf("a guess is %d letters a-z, got %q", WordLength, text)}
	}
	word := strings.ToLower(trimmed)
	if !words.Allowed().Contains(word) {
		return "", game.IllegalActionError{Msg: "not in word list: " + word}
	}
	return word, nil
}

func isNotASCIILetter(r rune) bool {
	return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
}

func (s state) with(latest Guess) state {
	next := state{answer: s.answer, guesses: append(slices.Clip(s.guesses), latest)}
	switch {
	case latest.Feedback.solved():
		next.result = game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonSolved}
		next.finished = true
	case len(next.guesses) == MaxGuesses:
		next.result, next.finished = game.ForfeitToRemaining([]game.Seat{SeatSolver}, SeatSolver, ReasonOutOfGuesses), true
	}
	return next
}

func (s state) View(game.Seat) View {
	view := View{
		Guesses:   append([]Guess{}, s.guesses...),
		Keyboard:  keyboardOf(s.guesses),
		Remaining: MaxGuesses - len(s.guesses),
	}
	if s.finished {
		view.Answer = s.answer
	}
	return view
}

func (s state) ToAct() []game.Seat {
	if s.finished {
		return nil
	}
	return []game.Seat{SeatSolver}
}

func (s state) Outcome() (game.Result, bool) { return s.result, s.finished }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining([]game.Seat{SeatSolver}, seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game and reveals the answer.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	if s.finished {
		return s
	}
	s.result, s.finished = result, true
	return s
}
