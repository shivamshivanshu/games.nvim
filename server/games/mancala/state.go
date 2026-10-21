package mancala

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type state struct {
	board    board
	toMove   game.Seat
	result   game.Result
	finished bool
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if !slices.Contains(s.ToAct(), seat) {
		return nil, nil, game.ErrNotYourTurn
	}
	if action.Kind != ActionSow {
		return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
	}
	pit, err := parsePit(action.Pit)
	if err != nil {
		return nil, nil, err
	}
	if s.board[pitHole(seat, pit)] == 0 {
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("pit %d is empty", pit)}
	}
	sown, last := s.board.sown(seat, pit)
	b, captured := sown.captured(seat, last)
	next := state{board: b, toMove: opponent(seat)}
	if b.pitsEmpty(SeatSouth) || b.pitsEmpty(SeatNorth) {
		ended, collected := next.ended()
		return ended, append([]game.Event{SowedEvent{By: seat, Pit: pit, Captured: captured}}, collected...), nil
	}
	again := last == storeHole(seat)
	if again {
		next.toMove = seat
	}
	return next, []game.Event{SowedEvent{By: seat, Pit: pit, Captured: captured, Again: again}}, nil
}

// ended gives each seat the seeds left in its pits and decides the game by the stores.
func (s state) ended() (state, []game.Event) {
	events := []game.Event{}
	for _, seat := range []game.Seat{SeatSouth, SeatNorth} {
		var seeds int
		s.board, seeds = s.board.collected(seat)
		if seeds > 0 {
			events = append(events, CollectedEvent{By: seat, Seeds: seeds})
		}
	}
	south, north := s.board[storeHole(SeatSouth)], s.board[storeHole(SeatNorth)]
	switch {
	case south > north:
		s.result = game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSouth}, Reason: ReasonMoreSeeds}
	case north > south:
		s.result = game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatNorth}, Reason: ReasonMoreSeeds}
	default:
		s.result = game.Result{Kind: game.ResultDraw, Reason: ReasonEqualSeeds}
	}
	s.finished = true
	return s, events
}

func (s state) View(viewer game.Seat) View {
	legal := []int{}
	if slices.Contains(s.ToAct(), viewer) {
		legal = s.board.legalPits(viewer)
	}
	return View{South: s.board.side(SeatSouth), North: s.board.side(SeatNorth), LegalPits: legal}
}

func (s state) ToAct() []game.Seat {
	if s.finished {
		return nil
	}
	return []game.Seat{s.toMove}
}

func (s state) Outcome() (game.Result, bool) { return s.result, s.finished }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining([]game.Seat{SeatSouth, SeatNorth}, seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game, so no seat is offered
// a pit.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	if s.finished {
		return s
	}
	s.result, s.finished = result, true
	return s
}
