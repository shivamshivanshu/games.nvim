package wordle

const alphabetSize = 26

// score marks each letter of guess against answer, both WordLength lowercase letters.
// Hits are marked first; then, left to right, a letter is present only while the
// answer still has an unmatched copy of it, so repeated letters are never over-counted.
func score(guess, answer string) Feedback {
	var feedback Feedback
	var unmatched [alphabetSize]int
	for i := range WordLength {
		if guess[i] == answer[i] {
			feedback[i] = MarkHit
			continue
		}
		unmatched[answer[i]-'a']++
	}
	for i := range WordLength {
		if feedback[i] == MarkHit {
			continue
		}
		letter := guess[i] - 'a'
		if unmatched[letter] == 0 {
			feedback[i] = MarkMiss
			continue
		}
		feedback[i] = MarkPresent
		unmatched[letter]--
	}
	return feedback
}

func (f Feedback) solved() bool {
	for _, mark := range f {
		if mark != MarkHit {
			return false
		}
	}
	return true
}
