package hub

import "github.com/shivamshivanshu/games.nvim/server/game"

// PauseGamesOf sends the pauses a user's last session sets off on closing, so
// a test can have them arrive after the user is back.
func (h *Hub) PauseGamesOf(user game.UserID) { h.pauseGamesOf(user) }
