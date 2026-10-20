package games

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
	"github.com/shivamshivanshu/games.nvim/server/games/tictactoe"
)

func All() []game.Game {
	return []game.Game{game.Adapt(chess.Rules{}), game.Adapt(tictactoe.Rules{})}
}
