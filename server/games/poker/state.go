package poker

import (
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

// state is the whole table. Its arrays make every copy independent, so a
// value receiver can change its copy and return it without touching the original.
type state struct {
	n     int
	users [maxPlayers]game.UserID
	// out marks seats that busted or left; a seat all-in in the current hand is not out yet.
	out     [maxPlayers]bool
	subseed uint64
	hand    hand
	ended   endedHand
	game.Ending
}

// seatNames are the seats clockwise from p1.
var seatNames = func() (names [maxPlayers]game.Seat) {
	for i := range names {
		names[i] = game.NumberedSeat(i)
	}
	return names
}()

func seatName(i int) game.Seat { return seatNames[i] }

func (s state) seatIndex(seat game.Seat) (int, bool) {
	i := slices.Index(seatNames[:s.n], seat)
	return i, i >= 0
}

func (s state) aliveCount() int { return xslices.Count(s.out[:s.n], false) }

// nextAlive is the first seat after i, clockwise, still in the game.
func (s state) nextAlive(i int) int {
	return game.NextSeat(s.n, i, func(j int) bool { return !s.out[j] })
}

// aliveFrom lists the seats still in the game clockwise, starting at start.
func (s state) aliveFrom(start int) []int {
	var order []int
	for k := range s.n {
		if i := (start + k) % s.n; !s.out[i] {
			order = append(order, i)
		}
	}
	return order
}

// Apply plays the betting move of the seat to act, then runs the table on to the next decision.
func (s state) Apply(seat game.Seat, a Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if a.Kind != ActionAct {
		return nil, nil, game.Illegalf("unknown action %q: want %q", a.Kind, ActionAct)
	}
	i := s.hand.toAct
	m, err := parseMove(a.Text)
	if err != nil {
		return nil, nil, err
	}
	chips, played, err := s.hand.chips.apply(i, m)
	if err != nil {
		return nil, nil, err
	}
	acted := ActedEvent{By: seat, Move: played}
	if played != MoveFold && played != MoveCheck {
		acted.Amount = chips.bet[i]
	}
	s.hand.chips = chips
	s.hand.toAct = (i + 1) % s.n
	next, events := s.advance(seat, ReasonAllChips)
	return next, append([]game.Event{acted}, events...), nil
}

// Eliminate folds seat out of the hand and out of the game. Chips it has put
// in stay in the pots as dead money; the rest of its stack is forfeited and
// leaves play. So the chips in play, stacks and pots, plus the forfeited
// stacks always total startingStack per player.
func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	i, ok := s.seatIndex(seat)
	if !ok || s.Over() || s.out[i] {
		return s, nil
	}
	s.out[i] = true
	s.hand.chips.stack[i] = 0
	s.hand.chips.contesting[i] = false
	ending := ReasonAllChips
	if s.aliveCount() == 1 {
		ending = reason
	}
	next, events := s.advance(seat, ending)
	return next, append([]game.Event{LeftEvent{By: seat}}, events...)
}

// ToAct is the one seat to act, or none once the game is over.
func (s state) ToAct() []game.Seat { return s.ToActOf(seatName(s.hand.toAct)) }

// Conclude ends the game on a result the core imposed, such as an abort, so
// that no seat is left to act.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Ending = s.Concluded(result)
	return s
}

// KnockedOut are the seats that busted or left.
func (s state) KnockedOut() []game.Seat {
	out := []game.Seat{}
	for i := range s.n {
		if s.out[i] {
			out = append(out, seatName(i))
		}
	}
	return out
}

// Scores are chip deltas at the finish. The winner holds every chip at the
// table, including stacks forfeited by seats that left, so they sum to zero.
// An ending without a winner, such as an abort, moves no chips.
func (s state) Scores() map[game.Seat]int64 {
	result, _ := s.Outcome()
	loss := -int64(startingStack)
	if result.Kind != game.ResultDecisive {
		loss = 0
	}
	scores := make(map[game.Seat]int64, s.n)
	for i := range s.n {
		scores[seatName(i)] = loss
	}
	for _, winner := range result.Winners {
		scores[winner] += int64(s.n) * int64(startingStack)
	}
	return scores
}

// View shows seat its own hole cards and nobody else's.
func (s state) View(seat game.Seat) View {
	h := s.hand
	v := View{
		HandNo:   h.number,
		Button:   seatName(h.button),
		Blinds:   h.blinds,
		Board:    slices.Clone(h.board[:h.dealt]),
		Pots:     s.potViews(),
		Seats:    s.seatViews(),
		Legal:    []MoveKind{},
		MyCards:  []cards.Card{},
		LastHand: s.ended.summary(),
	}
	i, ok := s.seatIndex(seat)
	if !ok {
		return v
	}
	if h.dealtIn[i] {
		v.MyCards = slices.Clone(h.holes[i][:])
	}
	if !s.Over() && i == h.toAct {
		v.Legal, v.ToCall = h.chips.legalMoves(i), h.chips.toCall(i)
		if slices.ContainsFunc(v.Legal, raisesTo) {
			v.MinRaiseTo, v.MaxRaiseTo = h.chips.minRaiseTo(), h.chips.matchable(i)
		}
	}
	return v
}

func (s state) seatViews() []SeatView {
	chips := s.hand.chips
	seats := make([]SeatView, s.n)
	for i := range seats {
		seats[i] = SeatView{
			Seat:   seatName(i),
			User:   s.users[i],
			Stack:  chips.stack[i],
			Bet:    chips.bet[i],
			Folded: s.hand.dealtIn[i] && !chips.contesting[i],
			AllIn:  chips.allIn(i),
			Out:    s.out[i],
		}
	}
	return seats
}

// potViews are the pots settled on earlier streets.
func (s state) potViews() []PotView {
	chips := s.hand.chips
	var settled [maxPlayers]Chips
	for i := range s.n {
		settled[i] = chips.committed[i] - chips.bet[i]
	}
	pots := layerPots(settled[:s.n], chips.contesting[:s.n])
	views := make([]PotView, len(pots))
	for k, p := range pots {
		eligible := make([]game.Seat, len(p.eligible))
		for j, i := range p.eligible {
			eligible[j] = seatName(i)
		}
		views[k] = PotView{Amount: p.amount, Eligible: eligible}
	}
	return views
}
