package fifteen

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// Rules is the solo sliding puzzle: put the tiles in order from a seeded, solvable shuffle.
type Rules struct{}

const (
	tileField      = "tile"
	directionField = "direction"
)

// EventTypes implements game.EventCatalog.
func (Rules) EventTypes() []game.Event { return []game.Event{SlidEvent{}} }

// Reasons implements game.ReasonCatalog.
func (Rules) Reasons() []game.ResultReason { return []game.ResultReason{ReasonSolved} }

// Info describes the 15-puzzle to the core.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeFifteen,
		Name:         "15-puzzle",
		Players:      game.PlayerRange{Min: 1, Max: 1},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Options: []game.OptionSpec{{
			Name:    OptionSize,
			Values:  game.OptionValues(Size3, Size4),
			Default: game.OptionValue(Size4),
		}},
		Actions: []game.ActionSpec{
			{
				Kind:        ActionSlide,
				Description: "Slide a tile next to the blank into it.",
				Input:       &game.ActionInput{Field: tileField, Description: `The tile's number, e.g. "7".`, Pattern: tilePattern},
			},
			{
				Kind:        ActionMove,
				Description: "Slide the tile that moves this way into the blank: up moves the tile below it.",
				Input:       &game.ActionInput{Field: directionField, Description: "The way the tile moves.", Values: grid.DirectionNames()},
			},
		},
	}
}

// New seats the challenger alone and shuffles the tiles from rng.
func (Rules) New(opts Opts, challenger game.UserID, _ []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	side, ok := opts.Size.side()
	if !ok {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("unknown size %q", opts.Size)}
	}
	return state{board: shuffled(side, rng.Perm)}, game.SoloSeat(challenger), nil
}

func (s Size) side() (int, bool) {
	switch s {
	case Size3:
		return 3, true
	case Size4:
		return 4, true
	}
	return 0, false
}
