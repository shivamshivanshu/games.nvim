package wordle

import "github.com/shivamshivanshu/games.nvim/server/game/feedback"

func (f Feedback) solved() bool {
	for _, mark := range f {
		if mark != feedback.Hit {
			return false
		}
	}
	return true
}
