package chesspuzzle

import (
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
)

// MaxPlies is the longest line a game takes.
const MaxPlies = 600

// longestMoveText is the widest move either notation writes: SAN with a full
// square disambiguation, a capture and a check. UCI is at most 5 characters.
const longestMoveText = "Qh4xe1+"

// movesMaxLen fits MaxPlies of the widest moves, one space apart.
const movesMaxLen = MaxPlies*(len(longestMoveText)+len(" ")) - len(" ")

// fenMaxLen is above the longest legal FEN: 71 characters of placement, 13 of
// the other fields, and room for long move counters.
const fenMaxLen = 100

// line is the expected game from its start position, normalized once so that
// Apply compares moves and never parses the text again.
type line struct {
	start chess.Position
	moves []chess.Move
	san   []string
}

// parseLine replays text from fen, the standard start when empty. Any failure
// is a BadRequestError, naming the 1-based ply for a move that does not replay.
func parseLine(fen, text string) (line, error) {
	start, err := startPosition(fen)
	if err != nil {
		return line{}, err
	}
	fields := strings.Fields(text)
	if len(fields) == 0 || len(fields) > MaxPlies {
		return line{}, game.BadRequestf("the line holds %d plies, want 1 to %d", len(fields), MaxPlies)
	}
	parsed := line{start: start, moves: make([]chess.Move, 0, len(fields)), san: make([]string, 0, len(fields))}
	position := start
	for i, field := range fields {
		move, err := position.ParseMove(field)
		if err != nil {
			return line{}, game.BadRequestf("ply %d: %v", i+1, err)
		}
		parsed.moves = append(parsed.moves, move)
		parsed.san = append(parsed.san, position.SAN(move))
		position = position.Play(move)
	}
	return parsed, nil
}

func startPosition(fen string) (chess.Position, error) {
	if fen == "" {
		fen = chess.StartFEN
	}
	start, err := chess.ParseFEN(fen)
	if err != nil {
		return chess.Position{}, game.BadRequestError{Msg: err.Error()}
	}
	return start, nil
}

// sideAt is the colour that plays the line's move at ply.
func (l line) sideAt(ply int) chess.Color {
	if ply%2 == 0 {
		return l.start.SideToMove
	}
	return l.start.SideToMove.Opponent()
}

// movesBy counts c's moves in the line from ply on.
func (l line) movesBy(c chess.Color, ply int) int {
	count := 0
	for ; ply < len(l.moves); ply++ {
		if l.sideAt(ply) == c {
			count++
		}
	}
	return count
}
