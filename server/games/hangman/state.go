package hangman

import (
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type state struct {
	game.Ending
	word        string
	guessed     []LetterGuess
	missedWords []string
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if action.Kind != ActionGuess {
		return nil, nil, game.UnknownAction(action.Kind)
	}
	guess, err := parseGuess(action.Text)
	if err != nil {
		return nil, nil, err
	}
	if s.alreadyGuessed(guess) {
		return nil, nil, game.Illegalf("already guessed: %s", guess)
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
		next.Ending = next.Concluded(game.WinFor(ReasonSolved, game.SeatSolo))
	case next.misses() == MaxMisses:
		next.Ending = next.Concluded(game.LossBy(ReasonHanged))
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
	result, over := s.Outcome()
	solved := over && result.Reason == ReasonSolved
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
	if s.Over() {
		view.Word = s.word
	}
	return view
}

func (s state) ToAct() []game.Seat {
	if s.Over() {
		return nil
	}
	return []game.Seat{game.SeatSolo}
}

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining([]game.Seat{game.SeatSolo}, seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game and reveals the word.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Ending = s.Concluded(result)
	return s
}
