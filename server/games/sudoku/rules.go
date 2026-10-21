package sudoku

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// Rules is solo Sudoku: fill the grid of a seeded puzzle that has a unique solution.
type Rules struct{}

const (
	textField = "text"
	cellField = "cell"
)

// Info describes Sudoku to the core.
func (Rules) Info() game.Info {
	return game.Info{
		Type:         TypeSudoku,
		Name:         "Sudoku",
		Players:      game.PlayerRange{Min: 1, Max: 1},
		Capabilities: []game.Capability{game.CapResign, game.CapAbort},
		Options: []game.OptionSpec{{
			Name:    OptionDifficulty,
			Values:  game.OptionValues(DifficultyEasy, DifficultyMedium, DifficultyHard),
			Default: game.OptionValue(DifficultyMedium),
		}},
		Actions: []game.ActionSpec{{
			Kind:        ActionSet,
			Description: "Write a digit into a cell that is not a given, or clear it with 0.",
			Input: &game.ActionInput{
				Field:       textField,
				Description: `Row, column, then the digit: "53 7" writes 7 at row 5, column 3; "53 0" clears it.`,
				Pattern:     setPattern,
			},
		}, {
			Kind:        ActionClear,
			Description: "Clear a cell that is not a given.",
			Input: &game.ActionInput{
				Field:       cellField,
				Description: `Row, then column: "53" clears row 5, column 3.`,
				Pattern:     clearPattern,
			},
		}},
		Events:  []game.Event{CellSetEvent{}},
		Reasons: []game.ResultReason{ReasonSolved},
	}
}

// New seats the challenger alone and generates the puzzle from rng.
func (Rules) New(opts Opts, challenger game.UserID, _ []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	givens, ok := opts.Difficulty.givens()
	if !ok {
		return nil, nil, game.BadRequestf("unknown difficulty %q", opts.Difficulty)
	}
	puzzle, solution := generate(givens, rng)
	return state{puzzle: puzzle, grid: puzzle, solution: solution}, game.SoloSeat(challenger), nil
}
