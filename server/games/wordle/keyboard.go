package wordle

import "github.com/shivamshivanshu/games.nvim/server/game/feedback"

// keyboardOf is the best Mark each letter of guesses has earned.
func keyboardOf(guesses []Guess) Keyboard {
	keyboard := Keyboard{}
	for _, guess := range guesses {
		for i, mark := range guess.Feedback {
			letter := guess.Word[i : i+1]
			if rank(mark) > rank(keyboard[letter]) {
				keyboard[letter] = mark
			}
		}
	}
	return keyboard
}

// rank orders marks from no information, through miss and present, to hit.
func rank(m Mark) int {
	switch m {
	case feedback.Miss:
		return 1
	case feedback.Present:
		return 2
	case feedback.Hit:
		return 3
	}
	return 0
}
