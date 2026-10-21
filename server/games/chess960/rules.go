package chess960

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
)

const moveDescription = "A move in SAN or UCI, any case, e.g. e4, Nf3, exd8=Q, e2e4; " +
	"castle as O-O, O-O-O or the king onto its own rook, e.g. e1h1."

// Rules implements game.Rules for Chess960: chess's, from a drawn start.
type Rules struct{}

// EventTypes implements game.EventCatalog with chess's events.
func (Rules) EventTypes() []game.Event { return chess.Rules{}.EventTypes() }

// Reasons implements game.ReasonCatalog with chess's reasons.
func (Rules) Reasons() []game.ResultReason { return chess.Rules{}.Reasons() }

// Info is chess's, under its own type, so its games keep their own stats.
func (Rules) Info() game.Info {
	info := chess.Rules{}.Info()
	info.Type, info.Name = TypeChess960, "Chess960"
	for i, action := range info.Actions {
		if action.Kind == chess.ActionMove {
			input := *action.Input
			input.Description = moveDescription
			info.Actions[i].Input = &input
		}
	}
	return info
}

// New draws the start position from rng, then the colours as chess does.
func (Rules) New(opts chess.Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[chess.Action, View], map[game.UserID]game.Seat, error) {
	number := StartPosition(rng.IntN(positionCount))
	start, err := chess.Chess960Position(backRank(number))
	if err != nil {
		return nil, nil, err
	}
	played, seats, err := chess.NewGame(start, opts, challenger, invitees, rng)
	if err != nil {
		return nil, nil, err
	}
	return state{chess: played, number: number, startFEN: start.FEN()}, seats, nil
}
