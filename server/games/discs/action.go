package discs

import (
	"fmt"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

// Opts are the options both games are created with.
type Opts struct {
	First       game.FirstMover  `json:"first"`
	TimeControl game.TimeControl `json:"time_control"`
}

// Options are the option specs behind Opts.
func Options() []game.OptionSpec {
	return []game.OptionSpec{game.FirstMoverOption(), game.TimeControlOption()}
}

// ActionPlace puts the mover's disc on the cell it names.
const ActionPlace game.ActionKind = "place"

const cellField = "cell"

// Action is a place on Cell, the cell's name as text, such as "h8".
type Action struct {
	Kind game.ActionKind `json:"kind"`
	Cell string          `json:"cell"`
}

// PlaceSpec is the catalogue entry of the place action on a board named by coords.
func PlaceSpec(coords grid.Coords, description string) game.ActionSpec {
	return game.ActionSpec{
		Kind:        ActionPlace,
		Description: description,
		Input: &game.ActionInput{
			Field:       cellField,
			Description: fmt.Sprintf("The cell, %s: the column's letter from a at the left, then the row's number from 1 at the top.", coords.Range()),
			Pattern:     coords.Pattern(),
		},
	}
}

// PlacedPoint is the point a place action names, or why the action is illegal.
func PlacedPoint(coords grid.Coords, action Action) (grid.Point, error) {
	if action.Kind != ActionPlace {
		return grid.Point{}, game.UnknownAction(action.Kind)
	}
	p, ok := coords.Parse(action.Cell)
	if !ok {
		return grid.Point{}, game.Illegalf("cell must be %s, a letter then a number, got %q", coords.Range(), action.Cell)
	}
	return p, nil
}
