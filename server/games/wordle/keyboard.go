package wordle

// keyboardOf is the best Mark each letter of guesses has earned.
func keyboardOf(guesses []Guess) Keyboard {
	keyboard := Keyboard{}
	for _, guess := range guesses {
		for i, mark := range guess.Feedback {
			letter := guess.Word[i : i+1]
			if mark.rank() > keyboard[letter].rank() {
				keyboard[letter] = mark
			}
		}
	}
	return keyboard
}

// rank orders marks from no information, through miss and present, to hit.
func (m Mark) rank() int {
	switch m {
	case MarkMiss:
		return 1
	case MarkPresent:
		return 2
	case MarkHit:
		return 3
	}
	return 0
}
