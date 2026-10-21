package reversi

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
	"github.com/shivamshivanshu/games.nvim/server/games/discs"
)

type state struct {
	discs.Turn
	board discs.Board
	last  *discs.Cell
}

func (s state) Apply(seat game.Seat, action discs.Action, _ *rand.Rand) (game.State[discs.Action, View], []game.Event, error) {
	if !slices.Contains(s.ToAct(), seat) {
		return nil, nil, game.ErrNotYourTurn
	}
	coords := s.board.Coords()
	at, err := discs.PlacedPoint(coords, action)
	if err != nil {
		return nil, nil, err
	}
	if s.board.At(at) != discs.NoDisc {
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("%s is taken", coords.Name(at))}
	}
	turned := flips(s.board, seat, at)
	if len(turned) == 0 {
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("%s flips no disc; a move must flip at least one", coords.Name(at))}
	}
	next, events := s.place(seat, at, turned)
	return next, events, nil
}

// place puts seat's disc on at, turns the flipped discs, and hands the move on:
// to the opponent if it can move, else back to seat with the opponent's pass,
// else the game ends on the disc count.
func (s state) place(seat game.Seat, at grid.Point, turned []grid.Point) (state, []game.Event) {
	last := discs.CellAt(at)
	next := state{Turn: s.Turn, board: s.board.With(seat, at).With(seat, turned...), last: &last}
	events := []game.Event{PlacedEvent{By: seat, Cell: last, Flipped: discs.CellsAt(turned)}}
	opponent := discs.Opponent(seat)
	switch {
	case len(legalPoints(next.board, opponent)) > 0:
		next.Turn = s.To(opponent)
	case len(legalPoints(next.board, seat)) > 0:
		next.Turn = s.To(seat)
		events = append(events, PassedEvent{By: opponent})
	default:
		next.Turn = s.Finish(countResult(next.board))
	}
	return next, events
}

func countResult(b discs.Board) game.Result {
	black, white := b.Count(discs.SeatBlack), b.Count(discs.SeatWhite)
	switch {
	case black > white:
		return game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{discs.SeatBlack}, Reason: ReasonMoreDiscs}
	case white > black:
		return game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{discs.SeatWhite}, Reason: ReasonMoreDiscs}
	}
	return game.Result{Kind: game.ResultDraw, Reason: ReasonEqualDiscs}
}

func (s state) View(viewer game.Seat) View {
	return View{
		Cells: s.board.Rows(),
		Last:  s.last,
		LegalCells: s.LegalFor(viewer, func() []grid.Point {
			return legalPoints(s.board, viewer)
		}),
		Discs: DiscCount{Black: s.board.Count(discs.SeatBlack), White: s.board.Count(discs.SeatWhite)},
	}
}

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[discs.Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining(discs.Seats(), seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game, so no seat is
// offered a cell.
func (s state) Conclude(result game.Result) game.State[discs.Action, View] {
	s.Turn = s.Turn.Conclude(result)
	return s
}
