package fifteen

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// tileSyntax is the catalogue's pattern for ActionSlide's tile: 1 to 15. Whether the tile
// exists depends on the size.
var tileSyntax = regexp.MustCompile(`^\s*([1-9]|1[0-5])\s*$`)

func parseTile(text string, b board) (int, error) {
	match := tileSyntax.FindStringSubmatch(text)
	if match == nil {
		return 0, game.Illegalf(`want a tile number, e.g. "7"; got %q`, text)
	}
	tile, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("tile %q: %w", match[1], err)
	}
	if tile >= len(b.tiles) {
		return 0, game.Illegalf("no tile %d on a %dx%d board", tile, b.side, b.side)
	}
	return tile, nil
}
