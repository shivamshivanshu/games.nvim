package chess960

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
)

// state is a chess game that remembers the start position it was drawn.
type state struct {
	chess    game.State[chess.Action, chess.View]
	number   StartPosition
	startFEN string
}

func (s state) with(played game.State[chess.Action, chess.View]) state {
	s.chess = played
	return s
}

func (s state) Apply(seat game.Seat, action chess.Action, rng *rand.Rand) (game.State[chess.Action, View], []game.Event, error) {
	played, events, err := s.chess.Apply(seat, action, rng)
	if err != nil {
		return nil, nil, err
	}
	return s.with(played), events, nil
}

func (s state) View(seat game.Seat) View {
	return View{View: s.chess.View(seat), StartPosition: s.number, StartFEN: s.startFEN}
}

func (s state) ToAct() []game.Seat { return s.chess.ToAct() }

func (s state) Outcome() (game.Result, bool) { return s.chess.Outcome() }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[chess.Action, View], []game.Event) {
	played, events := s.chess.Eliminate(seat, reason)
	return s.with(played), events
}

// Conclude implements game.Concluder with chess's, which every chess state is.
func (s state) Conclude(result game.Result) game.State[chess.Action, View] {
	return s.with(s.chess.(game.Concluder[chess.Action, chess.View]).Conclude(result))
}

// Exports implements game.Exporter with chess's PGN, which tags the variant.
func (s state) Exports(ctx game.ExportContext) map[string][]byte {
	return s.chess.(game.Exporter).Exports(ctx)
}
