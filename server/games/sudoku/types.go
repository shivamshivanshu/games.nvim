package sudoku

import "github.com/shivamshivanshu/games.nvim/server/game"

// TypeSudoku is the game type on the wire.
const TypeSudoku game.GameType = "sudoku"

// ReasonSolved ends a game whose grid equals the solution.
const ReasonSolved game.ResultReason = "solved"

// Difficulty sets how many givens the puzzle keeps; it means nothing else.
type Difficulty string

// The difficulties.
const (
	DifficultyEasy   Difficulty = "easy"
	DifficultyMedium Difficulty = "medium"
	DifficultyHard   Difficulty = "hard"
)

// OptionDifficulty names the difficulty option.
const OptionDifficulty game.OptionName = "difficulty"

// Opts are Sudoku's resolved options.
type Opts struct {
	Difficulty Difficulty `json:"difficulty"`
}

// ActionSet writes a digit into a cell, or clears it.
const ActionSet game.ActionKind = "set"

// Action is typed input; for ActionSet, Text is "RC D", e.g. "53 7" for row 5, column 3, digit 7.
type Action struct {
	Kind game.ActionKind `json:"kind"`
	Text string          `json:"text"`
}

// View is the grid in row-major order, with Empty for an unfilled cell. Conflicts marks
// every filled cell whose digit repeats in its row, column or box. Solution stays empty
// until the game finishes.
type View struct {
	Grid      Grid            `json:"grid"`
	Given     [CellCount]bool `json:"given"`
	Conflicts [CellCount]bool `json:"conflicts"`
	Solution  []Digit         `json:"solution"`
}

// CellSetEvent reports a cell written, or cleared when Digit is Empty; Row and Col are 1-based.
type CellSetEvent struct {
	By    game.Seat `json:"by"`
	Row   int       `json:"row"`
	Col   int       `json:"col"`
	Digit Digit     `json:"digit"`
}

// EventCellSet is the kind of CellSetEvent.
const EventCellSet game.EventKind = "cell_set"

// EventKind implements game.Event.
func (CellSetEvent) EventKind() game.EventKind { return EventCellSet }
