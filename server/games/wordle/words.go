package wordle

import (
	_ "embed"
	"sort"
	"strings"
)

//go:embed words/allowed.txt
var allowedText string

//go:embed words/answers.txt
var answersText string

const lineWidth = WordLength + 1

// wordList is a sorted list of WordLength-letter words, one per line, each line newline-terminated.
type wordList string

func allowedWords() wordList { return wordList(allowedText) }

func answerWords() wordList { return wordList(answersText) }

func (w wordList) len() int { return len(w) / lineWidth }

func (w wordList) at(i int) string {
	start := i * lineWidth
	return string(w[start : start+WordLength])
}

func (w wordList) contains(word string) bool {
	i := sort.Search(w.len(), func(i int) bool { return w.at(i) >= word })
	return i < w.len() && w.at(i) == word
}

func inWordList(word string) bool {
	return allowedWords().contains(word) || answerWords().contains(word)
}

func isWord(s string) bool {
	return len(s) == WordLength && !strings.ContainsFunc(s, func(r rune) bool { return r < 'a' || r > 'z' })
}
