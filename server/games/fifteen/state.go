package fifteen

import (
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

type state struct {
	game.Ending
	board board
	moves int
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	move, err := s.chosen(action)
	if err != nil {
		return nil, nil, err
	}
	next := state{board: s.board.slid(move.Tile), moves: s.moves + 1}
	if next.board.solved() {
		next.Ending = next.Concluded(game.WinFor(ReasonSolved, game.SeatSolo))
	}
	return next, []game.Event{SlidEvent{By: game.SeatSolo, Tile: move.Tile, Direction: move.Direction}}, nil
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
			return Movable{}, game.Illegalf("tile %d is not next to the blank", tile)
		}
		return movable[i], nil
	case ActionMove:
		direction, err := grid.ParseDirection(action.Direction)
		if err != nil {
			return Movable{}, err
		}
		i := slices.IndexFunc(movable, func(m Movable) bool { return m.Direction == direction })
		if i < 0 {
			return Movable{}, game.Illegalf("no tile can move %s", direction)
		}
		return movable[i], nil
	}
	return Movable{}, game.UnknownAction(action.Kind)
}

func (s state) View(game.Seat) View {
	view := View{Tiles: grid.RowsOf(s.board.tiles, s.board.side), Moves: s.moves, Movable: []Movable{}}
	if !s.Over() {
		view.Movable = s.board.movable()
	}
	return view
}

func (s state) ToAct() []game.Seat { return s.ToActOf(game.SeatSolo) }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining([]game.Seat{game.SeatSolo}, seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game and no tile is movable.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Ending = s.Concluded(result)
	return s
}
