// Package words holds the embedded English word lists that Wordle and Hangman draw from.
package words

import (
	_ "embed"
	"sort"
)

//go:embed allowed.txt
var allowedText string

//go:embed answers.txt
var answersText string

// Length is the number of letters in every listed word.
const Length = 5

const lineWidth = Length + 1

// List is a sorted list of Length-letter lowercase words, one per line, each line newline-terminated.
type List string

// Allowed is every word a guess may be.
func Allowed() List { return List(allowedText) }

// Answers is the common words a hidden word is drawn from; each is also in Allowed.
func Answers() List { return List(answersText) }

// Len is the number of words in w.
func (w List) Len() int { return len(w) / lineWidth }

// At is the i-th word of w, from 0.
func (w List) At(i int) string {
	start := i * lineWidth
	return string(w[start : start+Length])
}

// Contains reports whether word is in w.
func (w List) Contains(word string) bool {
	i := sort.Search(w.Len(), func(i int) bool { return w.At(i) >= word })
	return i < w.Len() && w.At(i) == word
}
