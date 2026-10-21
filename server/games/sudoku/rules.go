package sudoku

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// Rules is solo Sudoku: fill the grid of a seeded puzzle that has a unique solution.
type Rules struct{}

const textField = "text"

// EventTypes implements game.EventCatalog.
func (Rules) EventTypes() []game.Event { return []game.Event{CellSetEvent{}} }

// Reasons implements game.ReasonCatalog.
func (Rules) Reasons() []game.ResultReason { return []game.ResultReason{ReasonSolved} }

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
		}},
	}
}

// New seats the challenger alone and generates the puzzle from rng.
func (Rules) New(opts Opts, challenger game.UserID, invitees []game.UserID, rng *rand.Rand) (game.State[Action, View], map[game.UserID]game.Seat, error) {
	if len(invitees) != 0 {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("sudoku is solo, got %d opponents", len(invitees))}
	}
	givens, ok := opts.Difficulty.givens()
	if !ok {
		return nil, nil, game.BadRequestError{Msg: fmt.Sprintf("unknown difficulty %q", opts.Difficulty)}
	}
	puzzle, solution := generate(givens, rng)
	return state{puzzle: puzzle, grid: puzzle, solution: solution}, map[game.UserID]game.Seat{challenger: SeatSolver}, nil
}
