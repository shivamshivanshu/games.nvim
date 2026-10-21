package connect4

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

type state struct {
	board        board
	toMove       game.Seat
	last         *Cell
	winningLines [][]Cell
	result       game.Result
	finished     bool
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	switch action.Kind {
	case ActionDrop:
		return s.drop(seat, action.Column)
	}
	return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
}

func (s state) drop(seat game.Seat, input string) (game.State[Action, View], []game.Event, error) {
	column, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || column < 1 || column > boardColumns {
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("column must be 1 to %d, got %q", boardColumns, input)}
	}
	row, ok := s.board.landingRow(column - 1)
	if !ok {
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("column %d is full", column)}
	}
	at := grid.Point{Row: row, Col: column - 1}
	landed := cellAt(at)
	next := s
	next.board[at.Row][at.Col] = seat
	next.last = &landed
	next.toMove = sides.Other(seat)
	next.winningLines = winningLines(next.board, at)
	switch {
	case len(next.winningLines) > 0:
		next.result, next.finished = game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seat}, Reason: ReasonFourInARow}, true
	case next.board.full():
		next.result, next.finished = game.Result{Kind: game.ResultDraw, Reason: ReasonBoardFull}, true
	}
	return next, []game.Event{DroppedEvent{By: seat, Row: landed.Row, Column: landed.Column}}, nil
}

func winningLines(b board, last grid.Point) [][]Cell {
	lines := grid.WinningLines(b.cells(), last, winLength, noDisc)
	out := make([][]Cell, len(lines))
	for i, line := range lines {
		out[i] = make([]Cell, len(line))
		for j, p := range line {
			out[i][j] = cellAt(p)
		}
	}
	return out
}

func cellAt(p grid.Point) Cell {
	return Cell{Row: p.Row + 1, Column: p.Col + 1}
}

func (s state) View(viewer game.Seat) View {
	legal := []int{}
	if slices.Contains(s.ToAct(), viewer) {
		legal = s.board.openColumns()
	}
	return View{
		Cells:        s.board.cells(),
		Last:         s.last,
		WinningLines: s.winningLines,
		LegalColumns: legal,
	}
}

func (s state) ToAct() []game.Seat {
	if s.finished {
		return nil
	}
	return []game.Seat{s.toMove}
}

func (s state) Outcome() (game.Result, bool) { return s.result, s.finished }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining(sides.Seats(), seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game, so no seat is
// offered a drop.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	if s.finished {
		return s
	}
	s.result, s.finished = result, true
	return s
}
