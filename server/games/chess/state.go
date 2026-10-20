package chess

import (
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type state struct {
	position Position
	history  []RepetitionKey
	san      []string
	last     *MoveSquares
	result   game.Result
	finished bool
}

var allSeats = []game.Seat{SeatWhite, SeatBlack}

func newState(p Position) state {
	return state{position: p, history: []RepetitionKey{p.RepetitionKey()}}
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if !slices.Contains(s.ToAct(), seat) {
		return nil, nil, game.ErrNotYourTurn
	}
	switch action.Kind {
	case ActionMove:
		return s.applyMove(seat, action.Move)
	case ActionClaimDraw:
		return s.applyClaimDraw()
	}
	return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
}

func (s state) applyMove(seat game.Seat, input string) (game.State[Action, View], []game.Event, error) {
	legal := s.position.LegalMoves()
	move, err := s.position.parseMove(input, legal)
	if err != nil {
		return nil, nil, game.IllegalActionError{Msg: "illegal move: " + input}
	}
	san := s.position.san(move, legal)
	next := state{
		position: s.position.Play(move),
		san:      append(slices.Clip(s.san), san),
		last:     &MoveSquares{From: move.From.String(), To: move.To.String()},
	}
	nextLegal := next.position.LegalMoves()
	key := next.position.repetitionKey(nextLegal)
	next.history = append(slices.Clip(s.history), key)
	if ending, ok := DetectEnding(next.position, nextLegal, key, next.history); ok {
		next.result, next.finished = ending.result(), true
	}
	return next, []game.Event{MovedEvent{By: seat, SAN: san}}, nil
}

func (s state) applyClaimDraw() (game.State[Action, View], []game.Event, error) {
	reason, ok := ClaimableDraw(s.position, s.history)
	if !ok {
		return nil, nil, game.IllegalActionError{Msg: "no draw can be claimed"}
	}
	s.result, s.finished = game.Result{Kind: game.ResultDraw, Reason: reason}, true
	return s, nil, nil
}

func (e Ending) result() game.Result {
	result := game.Result{Kind: e.Kind, Reason: e.Reason}
	if e.Winner != nil {
		result.Winners = []game.Seat{seatOf(*e.Winner)}
	}
	return result
}

func (s state) View(game.Seat) View {
	return View{
		FEN:        s.position.FEN(),
		HistorySAN: append([]string{}, s.san...),
		LastMove:   s.last,
		InCheck:    s.position.InCheck(),
	}
}

func (s state) ToAct() []game.Seat {
	if s.finished {
		return nil
	}
	return []game.Seat{seatOf(s.position.SideToMove)}
}

func (s state) Outcome() (game.Result, bool) { return s.result, s.finished }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	if s.finished {
		return s, nil
	}
	s.result, s.finished = game.ForfeitToRemaining(allSeats, seat, reason), true
	return s, nil
}
