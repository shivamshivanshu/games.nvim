package chess

import (
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

type state struct {
	start    Position
	position Position
	legal    []Move
	history  []RepetitionKey
	san      []string
	last     *MoveSquares
	result   *game.Result
}

func newState(p Position) state {
	legal := p.LegalMoves()
	return state{start: p, position: p, legal: legal, history: []RepetitionKey{p.repetitionKey(legal)}}
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	switch action.Kind {
	case ActionMove:
		return s.applyMove(seat, action.Move)
	case ActionClaimDraw:
		return s.applyClaimDraw()
	}
	return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
}

func (s state) applyMove(seat game.Seat, input string) (game.State[Action, View], []game.Event, error) {
	move, err := s.position.parseMove(input, s.legal)
	if err != nil {
		return nil, nil, game.IllegalActionError{Msg: err.Error()}
	}
	position := s.position.Play(move)
	legal := position.LegalMoves()
	san := s.position.sanWithoutCheckSuffix(move, s.legal) + checkSuffix(position, legal)
	key := position.repetitionKey(legal)
	next := state{
		start:    s.start,
		position: position,
		legal:    legal,
		history:  append(slices.Clip(s.history), key),
		san:      append(slices.Clip(s.san), san),
		last:     &MoveSquares{From: move.From.String(), To: move.To.String()},
	}
	if ending, ok := DetectEnding(position, legal, key, next.history); ok {
		next.result = new(ending.result())
	}
	return next, []game.Event{MovedEvent{By: seat, SAN: san}}, nil
}

func (s state) applyClaimDraw() (game.State[Action, View], []game.Event, error) {
	reason, ok := s.position.claimableDraw(s.history[len(s.history)-1], s.history)
	if !ok {
		return nil, nil, game.IllegalActionError{Msg: "no draw can be claimed"}
	}
	s.result = &game.Result{Kind: game.ResultDraw, Reason: reason}
	return s, nil, nil
}

func (e Ending) result() game.Result {
	result := game.Result{Kind: e.Kind, Reason: e.Reason}
	if e.Winner != nil {
		result.Winners = []game.Seat{seatOf(*e.Winner)}
	}
	return result
}

func (s state) View(seat game.Seat) View {
	view := View{
		FEN:         s.position.FEN(),
		HistorySAN:  append([]string{}, s.san...),
		LastMove:    s.last,
		LegalMoves:  []string{},
		CheckSquare: s.position.CheckSquare(),
	}
	if slices.Contains(s.ToAct(), seat) {
		view.LegalMoves = xslices.Map(s.legal, Move.UCI)
		_, view.ClaimableDraw = s.position.claimableDraw(s.history[len(s.history)-1], s.history)
	}
	return view
}

func (s state) ToAct() []game.Seat {
	if s.result != nil {
		return nil
	}
	return []game.Seat{seatOf(s.position.SideToMove)}
}

func (s state) Outcome() (game.Result, bool) {
	if s.result == nil {
		return game.Result{}, false
	}
	return *s.result, true
}

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	if s.result != nil {
		return s, nil
	}
	s.result = new(game.ForfeitToRemaining(sides.Seats(), seat, reason))
	return s, nil
}

func (s state) Conclude(result game.Result) game.State[Action, View] {
	if s.result == nil {
		s.result = &result
	}
	return s
}
