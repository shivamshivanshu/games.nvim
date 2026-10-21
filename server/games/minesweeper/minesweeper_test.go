package minesweeper

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

const solo game.UserID = "alice"

// fixture is a board with its mines already placed: "*" is a mine, "." a safe cell.
func fixture(rows ...string) state {
	lvl := level{rows: len(rows), cols: len(rows[0])}
	for _, row := range rows {
		lvl.mines += strings.Count(row, "*")
	}
	s := newState(lvl)
	s.mines = make([]bool, lvl.cells())
	for r, row := range rows {
		for c, ch := range row {
			s.mines[r*lvl.cols+c] = ch == '*'
		}
	}
	return s
}

// oneMine has its mine at c4; revealing a1 opens all but c5, d4 and d5.
func oneMine() state {
	return fixture(
		".....",
		".....",
		"...*.",
		".....",
	)
}

func reveal(cell string) Action { return Action{Kind: ActionReveal, Cell: cell} }
func flag(cell string) Action   { return Action{Kind: ActionFlag, Cell: cell} }
func chord(cell string) Action  { return Action{Kind: ActionChord, Cell: cell} }

func mustApply(t *testing.T, s state, actions ...Action) (state, []game.Event) {
	t.Helper()
	var events []game.Event
	for _, action := range actions {
		s, events = gametest.MustApply(t, s, game.SeatSolo, action, game.StepRNG(1, 1))
	}
	return s, events
}

// drawn is the view's cells one row to a string: "#" hidden, "F" flag, a digit for a revealed
// count, and once over "*" a mine, "M" a flagged mine, "X" the exploded one and "!" a wrong flag.
func drawn(v View) []string {
	glyph := map[Cell]string{CellHidden: "#", CellFlag: "F", CellMine: "*", CellFlaggedMine: "M", CellExploded: "X", CellWrongFlag: "!"}
	rows := make([]string, len(v.Cells))
	for r, row := range v.Cells {
		var b strings.Builder
		for _, cell := range row {
			if g, ok := glyph[cell]; ok {
				b.WriteString(g)
			} else {
				b.WriteString(string(cell))
			}
		}
		rows[r] = b.String()
	}
	return rows
}

func TestParseCell(t *testing.T) {
	easy, _ := DifficultyEasy.level()
	hard, _ := DifficultyHard.level()
	tests := []struct {
		lvl  level
		text string
		want int
	}{
		{easy, "a1", 0},
		{easy, " C5\t", 2*9 + 4},
		{easy, "i9", 80},
		{hard, "p30", 15*30 + 29},
		{hard, "a12", 11},
	}
	for _, tt := range tests {
		got, err := tt.lvl.parseCell(tt.text)
		if err != nil || got != tt.want {
			t.Errorf("parseCell(%q) = %d, %v; want %d", tt.text, got, err, tt.want)
		}
	}
	for _, text := range []string{"", "a0", "a31", "q1", "1a", "a 1", "aa1", "a01", "a1b"} {
		_, err := easy.parseCell(text)
		want := `want a row letter then a column number, e.g. "c12"; got "` + text + `"`
		if !errors.As(err, new(game.IllegalActionError)) || err.Error() != want {
			t.Errorf("parseCell(%q): got %v, want illegal action %q", text, err, want)
		}
	}
	for text, want := range map[string]string{
		"j1":  "no cell j1 on a board of 9 rows by 9 columns",
		"a10": "no cell a10 on a board of 9 rows by 9 columns",
	} {
		if _, err := easy.parseCell(text); !errors.As(err, new(game.IllegalActionError)) || err.Error() != want {
			t.Errorf("parseCell(%q): got %v, want illegal action %q", text, err, want)
		}
	}
}

func TestDifficultiesSetTheBoardAndMines(t *testing.T) {
	tests := []struct {
		d          Difficulty
		rows, cols int
		mines      int
	}{
		{DifficultyEasy, 9, 9, 10},
		{DifficultyMedium, 16, 16, 40},
		{DifficultyHard, 16, 30, 99},
	}
	for _, tt := range tests {
		st, _, err := Rules{}.New(Opts{Difficulty: tt.d}, solo, nil, game.StepRNG(1, 0))
		if err != nil {
			t.Fatal(err)
		}
		view := st.View(game.SeatSolo)
		if len(view.Cells) != tt.rows || len(view.Cells[0]) != tt.cols || view.Remaining != tt.mines || view.Flags != 0 {
			t.Errorf("%s: %d rows by %d columns, remaining %d, flags %d; want %d by %d, remaining %d",
				tt.d, len(view.Cells), len(view.Cells[0]), view.Remaining, view.Flags, tt.rows, tt.cols, tt.mines)
		}
	}
	if _, _, err := (Rules{}).New(Opts{Difficulty: "huge"}, solo, nil, game.StepRNG(1, 0)); !errors.As(err, new(game.BadRequestError)) {
		t.Errorf("unknown difficulty: got %v, want a bad request", err)
	}
}

func TestFirstRevealAlwaysOpensAZeroWithTheMinesPlacedAfterIt(t *testing.T) {
	layouts := map[string]bool{}
	for _, d := range difficulties() {
		lvl, _ := d.level()
		for _, first := range []int{0, lvl.cells() - 1, lvl.cells() / 2, lvl.cols - 1} {
			for seed := range game.Seed(10) {
				st, _, err := Rules{}.New(Opts{Difficulty: d}, solo, nil, game.StepRNG(seed, 0))
				if err != nil {
					t.Fatal(err)
				}
				next, _, err := st.Apply(game.SeatSolo, reveal(address(lvl.point(first))), game.StepRNG(seed, 1))
				if err != nil {
					t.Fatal(err)
				}
				s := next.(state)
				p := lvl.point(first)
				if got := s.View(game.SeatSolo).Cells[p.Row][p.Col]; got != "0" || countSet(s.mines) != lvl.mines || s.Over() {
					t.Errorf("%s seed %d reveal %s: cell %q, %d mines, finished %t; want \"0\", %d mines, playing",
						d, seed, address(p), got, countSet(s.mines), s.Over(), lvl.mines)
				}
				layouts[string(d)+fmtMines(s)] = true
			}
		}
	}
	if want := len(difficulties()) * 4 * 10; len(layouts) < want-2 {
		t.Errorf("%d distinct mine layouts from %d games; the seed barely changes them", len(layouts), want)
	}
}

func fmtMines(s state) string {
	var b strings.Builder
	for _, mine := range s.mines {
		b.WriteByte(map[bool]byte{true: '*', false: '.'}[mine])
	}
	return b.String()
}

func TestRevealFloodsZerosToTheNumbersAroundThemAndStopsAtFlags(t *testing.T) {
	tests := []struct {
		name       string
		before     []Action
		action     Action
		wantCells  []string
		wantOpened int
	}{
		{
			name:       "a zero opens its whole area",
			action:     reveal("a1"),
			wantCells:  []string{"00000", "00111", "001##", "001##"},
			wantOpened: 16,
		},
		{
			name:       "a flag is never opened by a flood",
			before:     []Action{flag("a5")},
			action:     reveal("a1"),
			wantCells:  []string{"0000F", "00111", "001##", "001##"},
			wantOpened: 15,
		},
		{
			name:       "a number opens only itself",
			action:     reveal("d4"),
			wantCells:  []string{"#####", "#####", "#####", "###1#"},
			wantOpened: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := mustApply(t, oneMine(), tt.before...)
			next, events := mustApply(t, s, tt.action)
			if diff := cmp.Diff(tt.wantCells, drawn(next.View(game.SeatSolo))); diff != "" {
				t.Errorf("cells (-want +got):\n%s", diff)
			}
			p := next.lvl.point(mustParse(t, next.lvl, tt.action.Cell))
			want := []game.Event{RevealedEvent{By: game.SeatSolo, Row: p.Row + 1, Col: p.Col + 1, Opened: tt.wantOpened}}
			if diff := cmp.Diff(want, events); diff != "" {
				t.Errorf("events (-want +got):\n%s", diff)
			}
			if _, finished := next.Outcome(); finished {
				t.Error("game finished with safe cells left")
			}
		})
	}
}

func mustParse(t *testing.T, lvl level, text string) int {
	t.Helper()
	cell, err := lvl.parseCell(text)
	if err != nil {
		t.Fatal(err)
	}
	return cell
}

func TestOpeningEverySafeCellWins(t *testing.T) {
	s, _ := mustApply(t, oneMine(), flag("c4"), reveal("a1"), reveal("c5"), reveal("d4"))
	if _, finished := s.Outcome(); finished {
		t.Fatal("finished with d5 still hidden")
	}
	s, _ = mustApply(t, s, reveal("d5"))
	want := game.WinFor(ReasonCleared, game.SeatSolo)
	if got, finished := s.Outcome(); !finished || !cmp.Equal(got, want) {
		t.Errorf("outcome = %+v, %t; want %+v", got, finished, want)
	}
	view := s.View(game.SeatSolo)
	if diff := cmp.Diff([]string{"00000", "00111", "001M1", "00111"}, drawn(view)); diff != "" {
		t.Errorf("final cells (-want +got):\n%s", diff)
	}
	if len(view.Chordable) != 0 || len(s.ToAct()) != 0 {
		t.Errorf("finished game offers chords %v, to act %v", view.Chordable, s.ToAct())
	}
}

func TestRevealingAMineLosesAndShowsEveryMineFlaggedOrNotAndWrongFlags(t *testing.T) {
	s := fixture(
		"*....",
		".....",
		"...*.",
		".....",
	)
	s, _ = mustApply(t, s, reveal("d1"), flag("b5"), flag("a1"))
	s, events := mustApply(t, s, reveal("c4"))
	want := game.LossBy(ReasonMine)
	if got, finished := s.Outcome(); !finished || !cmp.Equal(got, want) {
		t.Errorf("outcome = %+v, %t; want %+v", got, finished, want)
	}
	if diff := cmp.Diff([]game.Event{RevealedEvent{By: game.SeatSolo, Row: 3, Col: 4}}, events); diff != "" {
		t.Errorf("events (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"M####", "111#!", "001X#", "001##"}, drawn(s.View(game.SeatSolo))); diff != "" {
		t.Errorf("final cells (-want +got):\n%s", diff)
	}
}

func TestAChordOnTwoWrongFlagsExplodesTheFirstMineItReaches(t *testing.T) {
	s := fixture(
		"*.*",
		"...",
		"...",
	)
	s, _ = mustApply(t, s, reveal("b2"), flag("b1"), flag("b3"), chord("b2"))
	if got, finished := s.Outcome(); !finished || got.Reason != ReasonMine {
		t.Fatalf("outcome = %+v, %t; want a loss to a mine", got, finished)
	}
	if want := mustParse(t, s.lvl, "a1"); s.exploded != want {
		t.Errorf("exploded cell %s, want a1, the first mine in the chord's row-major order", address(s.lvl.point(s.exploded)))
	}
	if diff := cmp.Diff([]string{"X2*", "!2!", "000"}, drawn(s.View(game.SeatSolo))); diff != "" {
		t.Errorf("final cells (-want +got):\n%s", diff)
	}
}

func TestFlagTogglesAndCountsAgainstTheMines(t *testing.T) {
	s, events := mustApply(t, oneMine(), flag("a1"))
	if diff := cmp.Diff([]game.Event{FlaggedEvent{By: game.SeatSolo, Row: 1, Col: 1, Flagged: true}}, events); diff != "" {
		t.Errorf("flag events (-want +got):\n%s", diff)
	}
	if v := s.View(game.SeatSolo); v.Flags != 1 || v.Remaining != 0 || v.Cells[0][0] != CellFlag {
		t.Errorf("after one flag: flags %d, remaining %d, a1 %q", v.Flags, v.Remaining, v.Cells[0][0])
	}
	s, _ = mustApply(t, s, flag("b2"))
	if v := s.View(game.SeatSolo); v.Flags != 2 || v.Remaining != -1 {
		t.Errorf("more flags than mines: flags %d, remaining %d; want 2, -1", v.Flags, v.Remaining)
	}
	s, events = mustApply(t, s, flag("a1"))
	if diff := cmp.Diff([]game.Event{FlaggedEvent{By: game.SeatSolo, Row: 1, Col: 1}}, events); diff != "" {
		t.Errorf("unflag events (-want +got):\n%s", diff)
	}
	if v := s.View(game.SeatSolo); v.Flags != 1 || v.Cells[0][0] != CellHidden {
		t.Errorf("after unflagging a1: flags %d, a1 %q", v.Flags, v.Cells[0][0])
	}
}

func TestChordRevealsTheUnflaggedNeighboursOfASatisfiedNumber(t *testing.T) {
	s, _ := mustApply(t, oneMine(), reveal("a1"), flag("c4"))
	wantChordable := []Square{{2, 4}, {2, 5}, {3, 3}, {4, 3}}
	if diff := cmp.Diff(wantChordable, s.View(game.SeatSolo).Chordable); diff != "" {
		t.Errorf("chordable (-want +got):\n%s", diff)
	}
	s, events := mustApply(t, s, chord("b5"))
	if diff := cmp.Diff([]game.Event{ChordedEvent{By: game.SeatSolo, Row: 2, Col: 5, Opened: 1}}, events); diff != "" {
		t.Errorf("events (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"00000", "00111", "001F1", "001##"}, drawn(s.View(game.SeatSolo))); diff != "" {
		t.Errorf("cells (-want +got):\n%s", diff)
	}
	s, _ = mustApply(t, s, chord("d3"), chord("c5"))
	if got, _ := s.Outcome(); got.Reason != ReasonCleared {
		t.Errorf("chording the last cells open: result %+v, want cleared", got)
	}
}

func TestChordOnAWrongFlagRevealsTheMineAndLoses(t *testing.T) {
	s, _ := mustApply(t, oneMine(), reveal("a1"), flag("c5"), chord("b5"))
	if got, finished := s.Outcome(); !finished || got.Reason != ReasonMine {
		t.Fatalf("outcome = %+v, %t; want a loss to a mine", got, finished)
	}
	if diff := cmp.Diff([]string{"00000", "00111", "001X!", "001##"}, drawn(s.View(game.SeatSolo))); diff != "" {
		t.Errorf("final cells (-want +got):\n%s", diff)
	}
}

func TestIllegalActionsAreRefusedWithAReason(t *testing.T) {
	tests := []struct {
		before []Action
		action Action
		want   string
	}{
		{nil, Action{Kind: "dig", Cell: "a1"}, "unknown action: dig"},
		{nil, reveal("z9"), `want a row letter then a column number, e.g. "c12"; got "z9"`},
		{nil, reveal("e1"), "no cell e1 on a board of 4 rows by 5 columns"},
		{[]Action{reveal("a1")}, reveal("a1"), "cell a1 is already revealed"},
		{[]Action{flag("a5")}, reveal("a5"), "cell a5 is flagged; take the flag off first"},
		{[]Action{reveal("a1")}, flag("b3"), "cell b3 is revealed"},
		{nil, chord("d4"), "cell d4 is not a revealed number"},
		{[]Action{reveal("a1")}, chord("a1"), "cell a1 is not a revealed number"},
		{[]Action{reveal("a1")}, chord("d3"), "cell d3 shows 1 but has 0 flags around it"},
		{[]Action{reveal("a1"), flag("c4"), flag("d4")}, chord("d3"), "cell d3 shows 1 but has 2 flags around it"},
		{[]Action{reveal("a1"), flag("c4")}, chord("b3"), "cell b3 has no hidden unflagged neighbour"},
	}
	for _, tt := range tests {
		s, _ := mustApply(t, oneMine(), tt.before...)
		_, _, err := s.Apply(game.SeatSolo, tt.action, nil)
		if !errors.As(err, new(game.IllegalActionError)) || err.Error() != tt.want {
			t.Errorf("%+v after %+v: got %v, want illegal action %q", tt.action, tt.before, err, tt.want)
		}
	}
}

func TestUnfinishedViewsHideEveryMine(t *testing.T) {
	s, _ := mustApply(t, oneMine(), reveal("a1"), flag("c4"), flag("d5"))
	for i, mine := range s.mines {
		p := s.lvl.point(i)
		if got := s.View(game.SeatSolo).Cells[p.Row][p.Col]; mine && got != CellHidden && got != CellFlag {
			t.Errorf("mine %s shows as %q before the end", address(p), got)
		}
	}
}

func TestGivingUpShowsTheMinesAndOffersNoMoreChords(t *testing.T) {
	s, _ := mustApply(t, oneMine(), reveal("a1"), flag("c4"), flag("d5"))
	resigned := game.LossBy(game.ReasonResign)
	over := s.Conclude(resigned)
	view := over.View(game.SeatSolo)
	if diff := cmp.Diff([]string{"00000", "00111", "001M#", "001#!"}, drawn(view)); diff != "" {
		t.Errorf("cells (-want +got):\n%s", diff)
	}
	if got, finished := over.Outcome(); !finished || !cmp.Equal(got, resigned) || len(view.Chordable) != 0 || len(over.ToAct()) != 0 {
		t.Errorf("outcome %+v, %t; chordable %v; to act %v", got, finished, view.Chordable, over.ToAct())
	}
	if again, _ := over.(state).Conclude(game.Result{Kind: game.ResultAborted, Reason: game.ReasonAborted}).Outcome(); !cmp.Equal(again, resigned) {
		t.Errorf("a second Conclude replaced the result with %+v", again)
	}
}

func TestGivingUpBeforeTheFirstRevealHasNoMinesToShow(t *testing.T) {
	s, _ := mustApply(t, newState(level{rows: 2, cols: 2, mines: 1}), flag("a1"))
	over := s.Conclude(game.LossBy(game.ReasonResign))
	if diff := cmp.Diff([]string{"F#", "##"}, drawn(over.View(game.SeatSolo))); diff != "" {
		t.Errorf("cells (-want +got):\n%s", diff)
	}
}

func TestViewArraysAreNeverNull(t *testing.T) {
	view := newState(level{rows: 2, cols: 2, mines: 1}).View(game.SeatSolo)
	if view.Chordable == nil {
		t.Error("chordable is nil, which encodes as null")
	}
}
