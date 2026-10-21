package fifteen

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

type state struct {
	board    board
	moves    int
	result   game.Result
	finished bool
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if s.finished || seat != SeatSolver {
		return nil, nil, game.ErrNotYourTurn
	}
	move, err := s.chosen(action)
	if err != nil {
		return nil, nil, err
	}
	next := state{board: s.board.slid(move.Tile), moves: s.moves + 1}
	if next.board.solved() {
		next.result = game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonSolved}
		next.finished = true
	}
	return next, []game.Event{SlidEvent{By: SeatSolver, Tile: move.Tile, Direction: move.Direction}}, nil
}

// chosen is the movable tile action names.
func (s state) chosen(action Action) (Movable, error) {
	movable := s.board.movable()
	switch action.Kind {
	case ActionSlide:
		tile, err := parseTile(action.Tile, s.board)
		if err != nil {
			return Movable{}, err
		}
		i := slices.IndexFunc(movable, func(m Movable) bool { return m.Tile == tile })
		if i < 0 {
			return Movable{}, game.IllegalActionError{Msg: fmt.Sprintf("tile %d is not next to the blank", tile)}
		}
		return movable[i], nil
	case ActionMove:
		direction, err := grid.ParseDirection(action.Direction)
		if err != nil {
			return Movable{}, err
		}
		i := slices.IndexFunc(movable, func(m Movable) bool { return m.Direction == direction })
		if i < 0 {
			return Movable{}, game.IllegalActionError{Msg: fmt.Sprintf("no tile can move %s", direction)}
		}
		return movable[i], nil
	}
	return Movable{}, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
}

func (s state) View(game.Seat) View {
	side := s.board.side
	view := View{Tiles: make([][]int, side), Moves: s.moves, Movable: []Movable{}}
	for row := range view.Tiles {
		view.Tiles[row] = slices.Clone(s.board.tiles[row*side : (row+1)*side])
	}
	if !s.finished {
		view.Movable = s.board.movable()
	}
	return view
}

func (s state) ToAct() []game.Seat {
	if s.finished {
		return nil
	}
	return []game.Seat{SeatSolver}
}

func (s state) Outcome() (game.Result, bool) { return s.result, s.finished }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining([]game.Seat{SeatSolver}, seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game and no tile is movable.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	if s.finished {
		return s
	}
	s.result, s.finished = result, true
	return s
}
