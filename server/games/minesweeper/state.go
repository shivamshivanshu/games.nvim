package minesweeper

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const noCell = -1

// state indexes cells row-major. mines is nil until the first reveal places them.
type state struct {
	game.Ending
	lvl      level
	mines    []bool
	open     []bool
	flags    []bool
	exploded int
}

func newState(lvl level) state {
	return state{lvl: lvl, open: make([]bool, lvl.cells()), flags: make([]bool, lvl.cells()), exploded: noCell}
}

func (s state) Apply(seat game.Seat, action Action, rng *rand.Rand) (game.State[Action, View], []game.Event, error) {
	apply, ok := map[game.ActionKind]func(int, *rand.Rand) (state, game.Event, error){
		ActionReveal: s.reveal,
		ActionFlag:   s.toggleFlag,
		ActionChord:  s.chord,
	}[action.Kind]
	if !ok {
		return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
	}
	cell, err := s.lvl.parseCell(action.Cell)
	if err != nil {
		return nil, nil, err
	}
	next, event, err := apply(cell, rng)
	if err != nil {
		return nil, nil, err
	}
	return next.settled(), []game.Event{event}, nil
}

func (s state) reveal(cell int, rng *rand.Rand) (state, game.Event, error) {
	switch {
	case s.open[cell]:
		return state{}, nil, s.illegal(cell, "is already revealed")
	case s.flags[cell]:
		return state{}, nil, s.illegal(cell, "is flagged; take the flag off first")
	}
	next := s.cloned()
	if next.mines == nil {
		next.mines = placeMines(s.lvl, cell, rng)
	}
	opened := next.uncover(cell)
	at := square(s.lvl.point(cell))
	return next, RevealedEvent{By: game.SeatSolo, Row: at.Row, Col: at.Col, Opened: opened}, nil
}

func (s state) toggleFlag(cell int, _ *rand.Rand) (state, game.Event, error) {
	if s.open[cell] {
		return state{}, nil, s.illegal(cell, "is revealed")
	}
	next := s.cloned()
	next.flags[cell] = !s.flags[cell]
	at := square(s.lvl.point(cell))
	return next, FlaggedEvent{By: game.SeatSolo, Row: at.Row, Col: at.Col, Flagged: next.flags[cell]}, nil
}

func (s state) chord(cell int, _ *rand.Rand) (state, game.Event, error) {
	if why := s.unchordable(cell); why != "" {
		return state{}, nil, s.illegal(cell, why)
	}
	next := s.cloned()
	opened := 0
	for _, n := range s.coveredAround(cell) {
		opened += next.uncover(n)
	}
	at := square(s.lvl.point(cell))
	return next, ChordedEvent{By: game.SeatSolo, Row: at.Row, Col: at.Col, Opened: opened}, nil
}

func (s state) illegal(cell int, why string) error {
	return game.IllegalActionError{Msg: fmt.Sprintf("cell %s %s", address(s.lvl.point(cell)), why)}
}

func (s state) cloned() state {
	s.mines, s.open, s.flags = slices.Clone(s.mines), slices.Clone(s.open), slices.Clone(s.flags)
	return s
}

// settled ends the game on an exploded mine, or once every safe cell is open.
func (s state) settled() state {
	switch {
	case s.exploded != noCell:
		s.Ending = s.Concluded(game.LossBy(ReasonMine))
	case s.mines != nil && countSet(s.open) == s.lvl.cells()-s.lvl.mines:
		s.Ending = s.Concluded(game.WinFor(ReasonCleared, game.SeatSolo))
	}
	return s
}

func (s state) View(game.Seat) View {
	view := View{Cells: make([][]Cell, s.lvl.rows), Chordable: []Square{}}
	for row := range view.Cells {
		view.Cells[row] = make([]Cell, s.lvl.cols)
	}
	for cell := range s.lvl.cells() {
		p := s.lvl.point(cell)
		view.Cells[p.Row][p.Col] = s.cellView(cell)
		if !s.Over() && s.unchordable(cell) == "" {
			view.Chordable = append(view.Chordable, square(p))
		}
	}
	view.Flags = countSet(s.flags)
	view.Remaining = s.lvl.mines - view.Flags
	return view
}

func (s state) cellView(cell int) Cell {
	switch {
	case s.Over() && cell == s.exploded:
		return CellExploded
	case s.Over() && s.isMine(cell) && s.flags[cell]:
		return CellFlaggedMine
	case s.Over() && s.isMine(cell):
		return CellMine
	case s.Over() && s.mines != nil && s.flags[cell]:
		return CellWrongFlag
	case s.open[cell]:
		return Cell(strconv.Itoa(s.minesAround(cell)))
	case s.flags[cell]:
		return CellFlag
	}
	return CellHidden
}

// unchordable says why a chord on cell is refused, or "" when it may chord.
func (s state) unchordable(cell int) string {
	count := s.minesAround(cell)
	switch flags := s.flagsAround(cell); {
	case !s.open[cell] || count == 0:
		return "is not a revealed number"
	case flags != count:
		return fmt.Sprintf("shows %d but has %d flags around it", count, flags)
	case len(s.coveredAround(cell)) == 0:
		return "has no hidden unflagged neighbour"
	}
	return ""
}

func (s state) ToAct() []game.Seat {
	if s.Over() {
		return nil
	}
	return []game.Seat{game.SeatSolo}
}

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining([]game.Seat{game.SeatSolo}, seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game and the view shows every mine.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Ending = s.Concluded(result)
	return s
}
