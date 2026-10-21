// Package feedback marks a guess against a hidden code symbol by symbol, as Wordle and Mastermind score.
package feedback

// Mark is the feedback for one symbol of a guess.
type Mark string

// The marks, from best to worst.
const (
	Hit     Mark = "hit"
	Present Mark = "present"
	Miss    Mark = "miss"
)

const symbolCount = 1 << 8

// Score marks each symbol of guess against code, which has the same length. Each byte is one
// symbol, so any alphabet of up to 256 symbols works. Hits are marked first; then, left to right,
// a symbol is present only while code still has an unmatched copy of it, so repeated symbols are
// never over-counted.
func Score(guess, code string) []Mark {
	marks := make([]Mark, len(guess))
	var unmatched [symbolCount]int
	for i := range len(guess) {
		if guess[i] == code[i] {
			marks[i] = Hit
			continue
		}
		unmatched[code[i]]++
	}
	for i := range len(guess) {
		if marks[i] == Hit {
			continue
		}
		symbol := guess[i]
		if unmatched[symbol] == 0 {
			marks[i] = Miss
			continue
		}
		marks[i] = Present
		unmatched[symbol]--
	}
	return marks
}
