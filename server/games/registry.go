package games

import (
	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/checkers"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
	"github.com/shivamshivanshu/games.nvim/server/games/connect4"
	"github.com/shivamshivanshu/games.nvim/server/games/ludo"
	"github.com/shivamshivanshu/games.nvim/server/games/tictactoe"
	"github.com/shivamshivanshu/games.nvim/server/games/wordle"
)

func All() []game.Game {
	return []game.Game{
		game.Adapt(chess.Rules{}),
		game.Adapt(tictactoe.Rules{}),
		game.Adapt(connect4.Rules{}),
		game.Adapt(checkers.Rules{}),
		game.Adapt(ludo.Rules{}),
		game.Adapt(wordle.Rules{}),
	}
}
