package wordle

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
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
	latest := Guess{Word: word, Feedback: score(word, s.answer)}
	return s.with(latest), []game.Event{GuessedEvent{By: seat, Word: latest.Word, Feedback: latest.Feedback}}, nil
}

func parseGuess(text string) (string, error) {
	word := strings.ToLower(strings.TrimSpace(text))
	if !isWord(word) {
		return "", game.IllegalActionError{Msg: fmt.Sprintf("a guess is %d letters a-z, got %q", WordLength, text)}
	}
	if !inWordList(word) {
		return "", game.IllegalActionError{Msg: "not in word list: " + word}
	}
	return word, nil
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
	view := View{Guesses: append([]Guess{}, s.guesses...), Remaining: MaxGuesses - len(s.guesses)}
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
	if s.finished {
		return s, nil
	}
	s.result, s.finished = game.ForfeitToRemaining([]game.Seat{SeatSolver}, seat, reason), true
	return s, nil
}
