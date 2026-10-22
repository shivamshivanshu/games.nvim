package wordle

import (
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/feedback"
	"github.com/shivamshivanshu/games.nvim/server/games/words"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

type state struct {
	game.Ending
	answer  string
	guesses []Guess
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if action.Kind != ActionGuess {
		return nil, nil, game.UnknownAction(action.Kind)
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
		return "", game.Illegalf("a guess is %d letters a-z, got %q", WordLength, text)
	}
	word := strings.ToLower(trimmed)
	if !words.Allowed().Contains(word) {
		return "", game.Illegalf("not in word list: %s", word)
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
		next.Ending = next.Concluded(game.WinFor(ReasonSolved, game.SeatSolo))
	case len(next.guesses) == MaxGuesses:
		next.Ending = next.Concluded(game.LossBy(ReasonOutOfGuesses))
	}
	return next
}

func (s state) View(game.Seat) View {
	view := View{
		Guesses:   xslices.Clone(s.guesses),
		Keyboard:  keyboardOf(s.guesses),
		Remaining: MaxGuesses - len(s.guesses),
	}
	if s.Over() {
		view.Answer = s.answer
	}
	return view
}

func (s state) ToAct() []game.Seat { return s.ToActOf(game.SeatSolo) }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining([]game.Seat{game.SeatSolo}, seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game and reveals the answer.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Ending = s.Concluded(result)
	return s
}
