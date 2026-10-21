package checkers

import (
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

// drawQuietPlies ends the game in a draw after this many plies in a row with
// no capture and no man move; a crowning is a man move.
const drawQuietPlies = 80

type state struct {
	game.Ending
	board      Board
	toMove     game.Seat
	legal      []Move
	quietPlies int
	history    []string
	last       []Square
}

func newState(b Board, toMove game.Seat, quietPlies int) state {
	s := state{board: b, toMove: toMove, legal: b.LegalMoves(toMove), quietPlies: quietPlies}
	if result, ended := s.ending(); ended {
		s.Ending = s.Concluded(result)
	}
	return s
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if action.Kind != ActionMove {
		return nil, nil, game.UnknownAction(action.Kind)
	}
	move, err := matchMove(action.Text, s.legal)
	if err != nil {
		return nil, nil, err
	}
	next, crowned := s.play(move)
	return next, []game.Event{MovedEvent{By: seat, Move: move.Text(), Crowned: crowned}}, nil
}

func (s state) play(m Move) (state, bool) {
	moved := s.board.At(m.From())
	board, crowned := s.board.Play(m)
	quietPlies := s.quietPlies + 1
	if m.IsCapture() || moved.isMan() {
		quietPlies = 0
	}
	next := newState(board, sides.Other(s.toMove), quietPlies)
	next.history = append(slices.Clip(s.history), m.Text())
	next.last = m.Path
	return next, crowned
}

// ending checks the side to move: with no legal move it has lost, which takes
// precedence over the quiet-ply draw.
func (s state) ending() (game.Result, bool) {
	winner := sides.Other(s.toMove)
	switch {
	case len(s.board.squaresOf(s.toMove)) == 0:
		return game.WinFor(ReasonNoPieces, winner), true
	case len(s.legal) == 0:
		return game.WinFor(ReasonNoMoves, winner), true
	case s.quietPlies >= drawQuietPlies:
		return game.DrawBy(ReasonFortyMove), true
	}
	return game.Result{}, false
}

func (s state) View(seat game.Seat) View {
	var legal []Move
	if slices.Contains(s.ToAct(), seat) {
		legal = s.legal
	}
	return View{
		Board:   s.board,
		Legal:   xslices.Map(legal, func(m Move) []Square { return slices.Clone(m.Path) }),
		Last:    xslices.Clone(s.last),
		History: xslices.Clone(s.history),
	}
}

func (s state) ToAct() []game.Seat {
	if s.Over() {
		return nil
	}
	return []game.Seat{s.toMove}
}

// Conclude takes a result the core imposed, so the game stops offering moves.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Ending = s.Concluded(result)
	return s
}

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	s.Ending = s.Concluded(game.ForfeitToRemaining(sides.Seats(), seat, reason))
	return s, nil
}
