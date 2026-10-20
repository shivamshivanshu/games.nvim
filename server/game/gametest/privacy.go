package gametest

import (
	"fmt"
	"reflect"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// Privacy names the strings each seat alone may see in a given state. Build it with Private.
type Privacy interface {
	privateTo(state any, seat game.Seat) ([]string, error)
}

type privateFunc[S any] func(state S, seat game.Seat) []string

// Private wraps fn, which reads the game's own state type S, as a Script.PrivateOf.
func Private[S any](fn func(state S, seat game.Seat) []string) Privacy {
	return privateFunc[S](fn)
}

func (f privateFunc[S]) privateTo(state any, seat game.Seat) ([]string, error) {
	typed, ok := state.(S)
	if !ok {
		return nil, fmt.Errorf("PrivateOf reads %v, but the game state is %T", reflect.TypeFor[S](), state)
	}
	return f(typed, seat), nil
}
