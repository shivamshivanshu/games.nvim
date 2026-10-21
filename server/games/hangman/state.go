package hangman

import (
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type state struct {
	word        string
	guessed     []LetterGuess
	missedWords []string
	result      game.Result
	finished    bool
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if action.Kind != ActionGuess {
		return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
	}
	guess, err := parseGuess(action.Text)
	if err != nil {
		return nil, nil, err
	}
	if s.alreadyGuessed(guess) {
		return nil, nil, game.IllegalActionError{Msg: "already guessed: " + guess}
	}
	next, hit := s.with(guess)
	return next, []game.Event{GuessedEvent{By: seat, Guess: guess, Hit: hit}}, nil
}

func (s state) alreadyGuessed(guess string) bool {
	return slices.Contains(s.missedWords, guess) ||
		slices.ContainsFunc(s.guessed, func(g LetterGuess) bool { return g.Letter == guess })
}

// with is the state after guess, a letter or a whole word, and whether it hit.
func (s state) with(guess string) (state, bool) {
	next := state{word: s.word, guessed: slices.Clip(s.guessed), missedWords: slices.Clip(s.missedWords)}
	var hit, solved bool
	if len(guess) == 1 {
		hit = strings.Contains(s.word, guess)
		next.guessed = append(next.guessed, LetterGuess{Letter: guess, Hit: hit})
		solved = hit && next.everyLetterFound()
	} else {
		hit = guess == s.word
		solved = hit
		if !hit {
			next.missedWords = append(next.missedWords, guess)
		}
	}
	switch {
	case solved:
		next.result = game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonSolved}
		next.finished = true
	case next.misses() == MaxMisses:
		next.result, next.finished = game.ForfeitToRemaining([]game.Seat{SeatSolver}, SeatSolver, ReasonHanged), true
	}
	return next, hit
}

func (s state) found(letter rune) bool {
	return slices.Contains(s.guessed, LetterGuess{Letter: string(letter), Hit: true})
}

func (s state) everyLetterFound() bool {
	return !strings.ContainsFunc(s.word, func(letter rune) bool { return !s.found(letter) })
}

func (s state) misses() int {
	missedLetters := 0
	for _, g := range s.guessed {
		if !g.Hit {
			missedLetters++
		}
	}
	return missedLetters + len(s.missedWords)
}

// pattern shows each letter of the word the solver has found, and the whole word once solved.
func (s state) pattern() []string {
	solved := s.finished && s.result.Reason == ReasonSolved
	pattern := make([]string, len(s.word))
	for i, letter := range s.word {
		if solved || s.found(letter) {
			pattern[i] = string(letter)
		}
	}
	return pattern
}

func (s state) View(game.Seat) View {
	view := View{
		Pattern:     s.pattern(),
		Guessed:     append([]LetterGuess{}, s.guessed...),
		MissedWords: append([]string{}, s.missedWords...),
		Misses:      s.misses(),
		Remaining:   MaxMisses - s.misses(),
	}
	if s.finished {
		view.Word = s.word
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

// Conclude keeps a result already reached; otherwise result ends the game and reveals the word.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	if s.finished {
		return s
	}
	s.result, s.finished = result, true
	return s
}
