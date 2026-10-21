package g2048

import (
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/grid"
)

type state struct {
	game.Ending
	board   board
	score   int
	reached bool
}

func (s state) Apply(seat game.Seat, action Action, rng *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if action.Kind != ActionMove {
		return nil, nil, game.UnknownAction(action.Kind)
	}
	direction, err := grid.ParseDirection(action.Direction)
	if err != nil {
		return nil, nil, err
	}
	slid, gained := s.board.slid(direction)
	if slid == s.board {
		return nil, nil, game.Illegalf("no tile can move %s", direction)
	}
	spawned, tile := slid.spawned(rng)
	next := state{board: spawned, score: s.score + gained, reached: s.reached || slid.contains(Goal)}
	events := []game.Event{MovedEvent{By: seat, Direction: direction, Gained: gained, Spawned: tile}}
	if next.reached && !s.reached {
		events = append(events, ReachedEvent{By: seat, Tile: Goal})
	}
	if len(next.board.movable()) == 0 {
		next.Ending = next.Concluded(noMovesResult(next.reached))
	}
	return next, events, nil
}

// noMovesResult is a win once the goal was reached, else a loss.
func noMovesResult(reached bool) game.Result {
	if reached {
		return game.WinFor(ReasonNoMoves, game.SeatSolo)
	}
	return game.LossBy(ReasonNoMoves)
}

func (s state) View(game.Seat) View {
	movable := []grid.Direction{}
	if !s.Over() {
		movable = s.board.movable()
	}
	return View{Cells: s.board.cells(), Score: s.score, Reached: s.reached, Movable: movable}
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

// Conclude keeps a result already reached; otherwise result ends the game and no move is offered.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Ending = s.Concluded(result)
	return s
}
