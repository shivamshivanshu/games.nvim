package games

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
)

func All() []game.Game {
	return []game.Game{game.Adapt(chess.Rules{})}
}
