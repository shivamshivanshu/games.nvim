package mancala

import (
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type state struct {
	game.Ending
	board  board
	toMove game.Seat
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if action.Kind != ActionSow {
		return nil, nil, game.UnknownAction(action.Kind)
	}
	pit, err := parsePit(action.Pit)
	if err != nil {
		return nil, nil, err
	}
	if s.board[pitHole(seat, pit)] == 0 {
		return nil, nil, game.Illegalf("pit %d is empty", pit)
	}
	sown, last := s.board.sown(seat, pit)
	b, captured := sown.captured(seat, last)
	next := state{board: b, toMove: sides.Other(seat)}
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
	for _, seat := range sides {
		var seeds int
		s.board, seeds = s.board.collected(seat)
		if seeds > 0 {
			events = append(events, CollectedEvent{By: seat, Seeds: seeds})
		}
	}
	south, north := s.board[storeHole(SeatSouth)], s.board[storeHole(SeatNorth)]
	switch {
	case south > north:
		s.Ending = s.Concluded(game.WinFor(ReasonMoreSeeds, SeatSouth))
	case north > south:
		s.Ending = s.Concluded(game.WinFor(ReasonMoreSeeds, SeatNorth))
	default:
		s.Ending = s.Concluded(game.DrawBy(ReasonEqualSeeds))
	}
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
	if s.Over() {
		return nil
	}
	return []game.Seat{s.toMove}
}

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining(sides.Seats(), seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game, so no seat is offered
// a pit.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Ending = s.Concluded(result)
	return s
}
