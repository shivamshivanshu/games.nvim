package g2048

import (
	"fmt"
	"math/rand/v2"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type state struct {
	board    board
	score    int
	reached  bool
	result   game.Result
	finished bool
}

func (s state) Apply(seat game.Seat, action Action, rng *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if s.finished || seat != SeatSolver {
		return nil, nil, game.ErrNotYourTurn
	}
	if action.Kind != ActionMove {
		return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
	}
	direction, err := parseDirection(action.Direction)
	if err != nil {
		return nil, nil, err
	}
	slid, gained := s.board.slid(direction)
	if slid == s.board {
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("no tile can move %s", direction)}
	}
	spawned, tile := slid.spawned(rng)
	next := state{board: spawned, score: s.score + gained, reached: s.reached || slid.contains(Goal)}
	events := []game.Event{MovedEvent{By: seat, Direction: direction, Gained: gained, Spawned: tile}}
	if next.reached && !s.reached {
		events = append(events, ReachedEvent{By: seat, Tile: Goal})
	}
	if len(next.board.movable()) == 0 {
		next.result, next.finished = noMovesResult(next.reached), true
	}
	return next, events, nil
}

// noMovesResult is a win once the goal was reached, else a loss.
func noMovesResult(reached bool) game.Result {
	winners := []game.Seat{}
	if reached {
		winners = []game.Seat{SeatSolver}
	}
	return game.Result{Kind: game.ResultDecisive, Winners: winners, Reason: ReasonNoMoves}
}

func (s state) View(game.Seat) View {
	movable := []Direction{}
	if !s.finished {
		movable = s.board.movable()
	}
	return View{Cells: s.board.cells(), Score: s.score, Reached: s.reached, Movable: movable}
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

// Conclude keeps a result already reached; otherwise result ends the game and no move is offered.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	if s.finished {
		return s
	}
	s.result, s.finished = result, true
	return s
}
