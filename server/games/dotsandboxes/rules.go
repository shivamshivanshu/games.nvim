// Package dotsandboxes is Dots and Boxes: two players draw edges between dots, and completing a
// box claims it and earns another move.
package dotsandboxes

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

const edgeField = "edge"

// Rules is Dots and Boxes on a square board of 3, 5 or 7 boxes a side; the most boxes wins.
type Rules struct{}

// EventTypes implements game.EventCatalog.
func (Rules) EventTypes() []game.Event { return []game.Event{DrawnEvent{}} }

// Reasons implements game.ReasonCatalog.
func (Rules) Reasons() []game.ResultReason {
	return []game.ResultReason{ReasonMoreBoxes, ReasonEqualBoxes}
}

// Info implements game.Rules.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeDotsAndBoxes,
		Name:         "Dots and Boxes",
		Players:      game.PlayerRange{Min: 2, Max: 2},
		Capabilities: []game.Capability{game.CapResign, game.CapDraw, game.CapClock, game.CapAbort},
		Options: []game.OptionSpec{
			{Name: OptionSize, Values: game.OptionValues(Size3, Size5, Size7), Default: game.OptionValue(Size5)},
			game.FirstMoverOption(),
			game.TimeControlOption(),
		},
		Actions: []game.ActionSpec{{
			Kind:        ActionDraw,
			Description: "Draw an undrawn edge between two adjacent dots; complete a box to claim it and move again.",
			Input: &game.ActionInput{
				Field:       edgeField,
				Description: "The edge: the dot at its left or top end, its column's letter from a at the left and its row's number from 1 at the top, then h lying across or v lying down, e.g. b2h.",
				Pattern:     catalogueEdges().Pattern(),
			},
		}},
	}
}

// New implements game.Rules. Seat p1 moves first: the challenger, or the opponent, as Opts.First
// picks.
func (Rules) New(opts Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	side := boxesPerSide(opts.Size)
	if side == 0 {
		return nil, nil, game.BadRequestf("unknown size %q", opts.Size)
	}
	return newState(side), sides.Assign(challenger, invitees[0], game.PickFirst(opts.First, rng)), nil
}

// boxesPerSide is the number size names, 0 for an unknown size.
func boxesPerSide(size Size) int {
	switch size {
	case Size3:
		return 3
	case Size5:
		return 5
	case Size7:
		return 7
	}
	return 0
}

// catalogueEdges names the edges of the largest board. Every board parses its actions with it,
// so the catalogue's pattern is exactly what the parser accepts, and refuses an edge off a
// smaller board as illegal on that board.
func catalogueEdges() grid.Edges {
	side := boxesPerSide(Size7)
	return grid.NewEdges(side, side)
}
