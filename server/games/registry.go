package games

import (
	"games.nvim/server/game"
	"games.nvim/server/games/chess"
)

func All() []game.Game {
	return []game.Game{game.Adapt(chess.Rules{})}
}
