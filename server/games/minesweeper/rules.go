package minesweeper

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// Rules is solo Minesweeper: reveal every safe cell without revealing a mine.
type Rules struct{}

const cellField = "cell"

// Info describes Minesweeper to the core.
func (Rules) Info() game.Info {
	cellInput := func() *game.ActionInput {
		return &game.ActionInput{
			Field:       cellField,
			Description: `Row letter, then column number: "c12" is row c, column 12.`,
			Pattern:     cellSyntax.String(),
		}
	}
	return game.Info{
		Type:         TypeMinesweeper,
		Name:         "Minesweeper",
		Players:      game.PlayerRange{Min: 1, Max: 1},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Options: []game.OptionSpec{{
			Name:    OptionDifficulty,
			Values:  game.OptionValues(DifficultyEasy, DifficultyMedium, DifficultyHard),
			Default: game.OptionValue(DifficultyEasy),
		}},
		Actions: []game.ActionSpec{
			{Kind: ActionReveal, Description: "Reveal a hidden cell that has no flag. The first reveal always opens an area.", Input: cellInput()},
			{Kind: ActionFlag, Description: "Flag a hidden cell, or take its flag off.", Input: cellInput()},
			{Kind: ActionChord, Description: "Reveal every unflagged neighbour of a revealed number that has as many flags around it.", Input: cellInput()},
		},
		Events:  []game.Event{RevealedEvent{}, FlaggedEvent{}, ChordedEvent{}},
		Reasons: []game.ResultReason{ReasonMine, ReasonCleared},
	}
}

// New seats the challenger alone on an empty board; the mines are placed by the first reveal.
func (Rules) New(opts Opts, challenger game.UserID, _ []game.UserID, _ *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	lvl, ok := opts.Difficulty.level()
	if !ok {
		return nil, nil, game.BadRequestf("unknown difficulty %q", opts.Difficulty)
	}
	return newState(lvl), game.SoloSeat(challenger), nil
}
