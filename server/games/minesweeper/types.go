package minesweeper

import "github.com/shivamshivanshu/games.nvim/server/game"

// TypeMinesweeper is the game type on the wire.
const TypeMinesweeper game.GameType = "minesweeper"

// The ways a game ends other than giving up.
const (
	ReasonMine    game.ResultReason = "mine"
	ReasonCleared game.ResultReason = "cleared"
)

// Difficulty sets the board's size and how many mines it hides.
type Difficulty string

// The difficulties: 9×9 with 10 mines, 16×16 with 40, and 16 rows by 30 columns with 99.
const (
	DifficultyEasy   Difficulty = "easy"
	DifficultyMedium Difficulty = "medium"
	DifficultyHard   Difficulty = "hard"
)

// OptionDifficulty names the difficulty option.
const OptionDifficulty game.OptionName = "difficulty"

// Opts are Minesweeper's resolved options.
type Opts struct {
	Difficulty Difficulty `json:"difficulty"`
}

// The actions, each on the cell its Cell text names.
const (
	ActionReveal game.ActionKind = "reveal"
	ActionFlag   game.ActionKind = "flag"
	ActionChord  game.ActionKind = "chord"
)

// Action names its cell as a row letter then a column number: "c12" is row 3, column 12.
type Action struct {
	Kind game.ActionKind `json:"kind"`
	Cell string          `json:"cell"`
}

// Cell is how one cell looks to the player: hidden, flagged, or revealed as the count
// "0" to "8" of the mines around it. Once the game is over every mine shows: CellExploded for
// the one revealed, CellFlaggedMine where a flag was right, CellMine where none was; a flag on
// a safe cell shows as CellWrongFlag.
type Cell string

// The cells that are not a revealed count.
const (
	CellHidden      Cell = "hidden"
	CellFlag        Cell = "flag"
	CellMine        Cell = "mine"
	CellFlaggedMine Cell = "flagged_mine"
	CellExploded    Cell = "exploded"
	CellWrongFlag   Cell = "wrong_flag"
)

// maxMinesAround is the most mines a cell can neighbour.
const maxMinesAround = 8

// EnumValues lists the cells that are not a count, then the counts.
func (Cell) EnumValues() []string {
	return append(game.Strings(CellHidden, CellFlag, CellMine, CellFlaggedMine, CellExploded, CellWrongFlag), append([]string{"0"}, game.NumberedValues(maxMinesAround)...)...)
}

// Square is a 1-based board position; row 1 is the top.
type Square struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// View holds Cells[row][col], rows from the top. Remaining is the mine count less the
// flags, negative when there are more flags than mines. Chordable lists the revealed
// numbers a chord may act on now, empty once the game is over.
type View struct {
	Cells     [][]Cell `json:"cells"`
	Flags     int      `json:"flags"`
	Remaining int      `json:"remaining"`
	Chordable []Square `json:"chordable"`
}

// RevealedEvent reports a reveal and how many cells it opened; Row and Col are 1-based.
type RevealedEvent struct {
	By     game.Seat `json:"by"`
	Row    int       `json:"row"`
	Col    int       `json:"col"`
	Opened int       `json:"opened"`
}

// FlaggedEvent reports a flag placed, or removed when Flagged is false.
type FlaggedEvent struct {
	By      game.Seat `json:"by"`
	Row     int       `json:"row"`
	Col     int       `json:"col"`
	Flagged bool      `json:"flagged"`
}

// ChordedEvent reports a chord on the number at Row, Col and how many cells it opened.
type ChordedEvent struct {
	By     game.Seat `json:"by"`
	Row    int       `json:"row"`
	Col    int       `json:"col"`
	Opened int       `json:"opened"`
}

// The event kinds.
const (
	EventRevealed game.EventKind = "revealed"
	EventFlagged  game.EventKind = "flagged"
	EventChorded  game.EventKind = "chorded"
)

// EventKind implements game.Event.
func (RevealedEvent) EventKind() game.EventKind { return EventRevealed }

// EventKind implements game.Event.
func (FlaggedEvent) EventKind() game.EventKind { return EventFlagged }

// EventKind implements game.Event.
func (ChordedEvent) EventKind() game.EventKind { return EventChorded }
