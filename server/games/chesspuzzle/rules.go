// Package chesspuzzle is solo chess along an imported line: the server plays
// the other side's moves and the solver finds their own.
package chesspuzzle

import (
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
)

const moveField = "text"

// Rules implements game.Rules for chess puzzles and study.
type Rules struct{}

func modes() []Mode { return []Mode{ModePuzzle, ModeStudy} }

func sides() []Side { return []Side{SideWhite, SideBlack} }

// Info describes chess puzzles: solo, with the line given in the options.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeChessPuzzle,
		Name:         "Chess puzzle",
		Players:      game.PlayerRange{Min: 1, Max: 1},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Options: []game.OptionSpec{
			{Name: OptionMode, Values: game.OptionValues(modes()...), Default: game.OptionValue(ModePuzzle)},
			game.TextOption(OptionFEN, fenMaxLen),
			game.TextOption(OptionMoves, movesMaxLen),
			{Name: OptionPlayer, Values: game.OptionValues(sides()...), Default: game.OptionValue(SideWhite)},
		},
		Actions: []game.ActionSpec{
			{
				Kind:        ActionMove,
				Description: "Play your move; in puzzle mode a wrong one ends the game.",
				Input: &game.ActionInput{
					Field:       moveField,
					Description: "A move in SAN or UCI, any case, e.g. e4, Nf3, O-O, exd8=Q, e2e4.",
					Pattern:     chess.MovePattern,
				},
			},
			{
				Kind:        ActionReveal,
				Description: "Play the line's move for you, counted as a miss; study mode only.",
			},
		},
		Events:  []game.Event{MovedEvent{}, RepliedEvent{}, MissedEvent{}, RevealedEvent{}},
		Reasons: []game.ResultReason{ReasonSolved, ReasonWrongMove, ReasonCompleted},
	}
}

// New checks the options and replays the line, then plays the other side's
// moves up to the solver's first.
func (Rules) New(opts Opts, challenger game.UserID, invitees []game.UserID, _ *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	if !slices.Contains(modes(), opts.Mode) {
		return nil, nil, game.BadRequestf("unknown mode %q", opts.Mode)
	}
	solver, ok := opts.Player.color()
	if !ok {
		return nil, nil, game.BadRequestf("unknown player %q", opts.Player)
	}
	parsed, err := parseLine(opts.FEN, opts.Moves)
	if err != nil {
		return nil, nil, err
	}
	if parsed.movesBy(solver, 0) == 0 {
		return nil, nil, game.BadRequestf("the line has no move for %s", opts.Player)
	}
	start, replies := newState(opts.Mode, solver, parsed).replyUntilSolverMoves()
	return opened{state: start, replies: replies}, game.SoloSeat(challenger), nil
}

// opened is the state New hands over, with the replies played to reach it.
type opened struct {
	state
	replies []game.Event
}

func (o opened) OpeningEvents() []game.Event { return o.replies }

func (s Side) color() (chess.Color, bool) {
	switch s {
	case SideWhite:
		return chess.White, true
	case SideBlack:
		return chess.Black, true
	}
	return 0, false
}
