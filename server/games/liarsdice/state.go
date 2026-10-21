package liarsdice

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/dice"
)

type state struct {
	order    []game.Seat
	cups     [][]dice.Face
	turn     int
	round    int
	bid      *Bid
	reveal   *Reveal
	result   game.Result
	finished bool
}

// rollCups rolls every cup that still holds dice, in seat order, each sorted.
func rollCups(sizes []int, rng *rand.Rand) [][]dice.Face {
	cups := make([][]dice.Face, len(sizes))
	for i, n := range sizes {
		cups[i] = dice.Roll(rng, n, dice.Six)
		slices.Sort(cups[i])
	}
	return cups
}

func newState(order []game.Seat, rng *rand.Rand) state {
	sizes := make([]int, len(order))
	for i := range sizes {
		sizes[i] = StartingDice
	}
	return state{order: order, cups: rollCups(sizes, rng), round: 1}
}

func (s state) totalDice() int {
	n := 0
	for _, cup := range s.cups {
		n += len(cup)
	}
	return n
}

func (s state) Apply(seat game.Seat, action Action, rng *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if !slices.Contains(s.ToAct(), seat) {
		return nil, nil, game.ErrNotYourTurn
	}
	switch action.Kind {
	case ActionBid:
		return s.applyBid(action.Bid)
	case ActionDudo:
		return s.applyDudo(rng)
	}
	return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
}

func (s state) applyBid(text string) (game.State[Action, View], []game.Event, error) {
	bid, err := parseBid(s.order[s.turn], text)
	if err != nil {
		return nil, nil, err
	}
	least, ok := minQuantity(s.bid, bid.Face)
	switch {
	case !ok:
		return nil, nil, game.IllegalActionError{Msg: "the opening bid cannot be on ones"}
	case bid.Quantity < least:
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("a bid on %ds needs at least %d dice", bid.Face, least)}
	case bid.Quantity > s.totalDice():
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("only %d dice are in play", s.totalDice())}
	}
	next := s
	next.bid = &bid
	next.turn = s.nextWithDice(s.turn)
	return next, []game.Event{BidEvent(bid)}, nil
}

// applyDudo reveals every cup: the bidder loses a die when fewer dice count
// than it bid, the challenger otherwise. The loser opens a new round with
// every cup rerolled, or the next seat with dice if the loser has none.
func (s state) applyDudo(rng *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if s.bid == nil {
		return nil, nil, game.IllegalActionError{Msg: "there is no bid to challenge: open the bidding"}
	}
	challenger := s.turn
	n := counted(s.cups, s.bid.Face)
	loser := challenger
	if n < s.bid.Quantity {
		loser = slices.Index(s.order, s.bid.By)
	}
	reveal := Reveal{Round: s.round, Bid: *s.bid, Challenger: s.order[challenger], Counted: n, Loser: s.order[loser], Cups: s.seatDice()}
	events := []game.Event{ChallengedEvent{By: reveal.Challenger, Bid: reveal.Bid, Counted: n, Loser: reveal.Loser, Cups: s.seatDice()}}

	sizes := make([]int, len(s.cups))
	for i, cup := range s.cups {
		sizes[i] = len(cup)
	}
	sizes[loser]--
	next := s
	next.reveal, next.bid = &reveal, nil
	if sizes[loser] == 0 {
		events = append(events, OutEvent{Seat: reveal.Loser})
	}
	next.cups = rollCups(sizes, rng)
	if winner, alone := next.lastWithDice(); alone {
		return next.finish(game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{winner}, Reason: ReasonLastWithDice}), events, nil
	}
	next.round++
	next.turn = loser
	if sizes[loser] == 0 {
		next.turn = next.nextWithDice(loser)
	}
	return next, events, nil
}

func (s state) seatDice() []SeatDice {
	out := make([]SeatDice, 0, len(s.cups))
	for i, cup := range s.cups {
		if len(cup) > 0 {
			out = append(out, SeatDice{Seat: s.order[i], Dice: slices.Clone(cup)})
		}
	}
	return out
}

// nextWithDice is the first seat after from, clockwise, that still has dice.
func (s state) nextWithDice(from int) int {
	for step := 1; step < len(s.order); step++ {
		if i := (from + step) % len(s.order); len(s.cups[i]) > 0 {
			return i
		}
	}
	return from
}

func (s state) lastWithDice() (game.Seat, bool) {
	var with []game.Seat
	for i, cup := range s.cups {
		if len(cup) > 0 {
			with = append(with, s.order[i])
		}
	}
	if len(with) != 1 {
		return "", false
	}
	return with[0], true
}

func (s state) finish(result game.Result) state {
	s.result, s.finished = result, true
	return s
}

func (s state) View(viewer game.Seat) View {
	me := slices.Index(s.order, viewer)
	toAct := slices.Contains(s.ToAct(), viewer)
	v := View{
		Round:      s.round,
		Seats:      make([]SeatView, len(s.order)),
		Bid:        cloned(s.bid),
		TotalDice:  s.totalDice(),
		MyDice:     []dice.Face{},
		MinBids:    []BidMinimum{},
		CanDudo:    toAct && s.bid != nil,
		LastReveal: s.revealView(),
	}
	for i, seat := range s.order {
		v.Seats[i] = SeatView{Seat: seat, Dice: len(s.cups[i]), Out: len(s.cups[i]) == 0}
	}
	if me >= 0 {
		v.MyDice = append(v.MyDice, s.cups[me]...)
	}
	if toAct {
		v.MinBids = s.minBids()
	}
	return v
}

func (s state) minBids() []BidMinimum {
	out := []BidMinimum{}
	for face := dice.Face(1); face <= dice.Six; face++ {
		if least, ok := minQuantity(s.bid, face); ok && least <= s.totalDice() {
			out = append(out, BidMinimum{Face: face, Quantity: least})
		}
	}
	return out
}

func cloned[T any](p *T) *T {
	if p == nil {
		return nil
	}
	c := *p
	return &c
}

func (s state) revealView() *Reveal {
	if s.reveal == nil {
		return nil
	}
	r := *s.reveal
	r.Cups = make([]SeatDice, len(s.reveal.Cups))
	for i, cup := range s.reveal.Cups {
		r.Cups[i] = SeatDice{Seat: cup.Seat, Dice: slices.Clone(cup.Dice)}
	}
	return &r
}

func (s state) ToAct() []game.Seat {
	if s.finished {
		return nil
	}
	return []game.Seat{s.order[s.turn]}
}

func (s state) Outcome() (game.Result, bool) { return s.result, s.finished }

// KnockedOut are the seats left without dice, by a challenge or by leaving.
func (s state) KnockedOut() []game.Seat {
	out := []game.Seat{}
	for i, cup := range s.cups {
		if len(cup) == 0 {
			out = append(out, s.order[i])
		}
	}
	return out
}

// Conclude takes a result the core imposed, such as an abort.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	if s.finished {
		return s
	}
	return s.finish(result)
}

// Eliminate takes the seat's dice off the table. The last seat with dice
// wins; otherwise the round's bidding starts over on the same dice, opened by
// the seat to act, or the next seat with dice if the leaver was to act.
func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	i := slices.Index(s.order, seat)
	if s.finished || i < 0 || len(s.cups[i]) == 0 {
		return s, nil
	}
	next := s
	next.cups = slices.Clone(s.cups)
	next.cups[i] = nil
	next.bid = nil
	if winner, alone := next.lastWithDice(); alone {
		return next.finish(game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{winner}, Reason: reason}), nil
	}
	if s.turn == i {
		next.turn = next.nextWithDice(i)
	}
	return next, nil
}
