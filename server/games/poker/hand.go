package poker

import (
	"cmp"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// endedHand is the hand that ended last, as it ended: its cards and seats,
// the hands shown and the pots won. Its slices are never changed once built.
type endedHand struct {
	hand   hand
	shown  []ShownHand
	awards []Award
}

// summary is a fresh copy for a view, or nil before any hand has ended.
func (e endedHand) summary() *HandSummary {
	if e.hand.number == 0 {
		return nil
	}
	return &HandSummary{
		HandNo: e.hand.number,
		Board:  slices.Clone(e.hand.board[:e.hand.dealt]),
		Shown:  cloneShown(e.shown),
		Awards: slices.Clone(e.awards),
	}
}

// hand is the hand being played, or at the finish the last one played.
type hand struct {
	number  int
	blinds  Blinds
	button  int
	dealtIn [maxPlayers]bool
	holes   [maxPlayers][holeCardsPerSeat]Card
	board   [boardSize]Card
	dealt   int
	chips   betting
	// toAct is the seat to act or, between moves, where the search for it starts.
	toAct int
}

// blindsFor doubles the blinds at the start of hands 11, 21, 31 and so on.
func blindsFor(handNo int) Blinds {
	level := (handNo - 1) / handsPerLevel
	return Blinds{Small: firstSmallBlind << level, Big: firstBigBlind << level}
}

// dealHand starts hand number with the button on seat button: it posts the
// blinds, deals from the hand's own deck and points to the first seat to act.
func (s state) dealHand(by game.Seat, number, button int) (state, []game.Event) {
	blinds := blindsFor(number)
	h := hand{number: number, blinds: blinds, button: button}
	chips := betting{n: s.n, stack: s.hand.chips.stack, bigBlind: blinds.Big}.newStreet()
	for i := range s.n {
		h.dealtIn[i] = !s.out[i]
		chips.contesting[i] = !s.out[i]
	}
	small, big := s.blindSeats(button)
	chips, smallPosted := chips.post(small, blinds.Small)
	chips, bigPosted := chips.post(big, blinds.Big)
	chips.currentBet = blinds.Big
	h.chips = chips
	h.holes, h.board = dealCards(shuffledDeck(s.subseed, number), s.aliveFrom(button+1))
	h.toAct = (big + 1) % s.n
	s.hand = h
	return s, []game.Event{
		HandStartedEvent{By: by, HandNo: number, Button: seatName(button), Blinds: blinds},
		PostedEvent{By: by, Seat: seatName(small), Blind: BlindSmall, Amount: smallPosted},
		PostedEvent{By: by, Seat: seatName(big), Blind: BlindBig, Amount: bigPosted},
	}
}

// blindSeats are left of the button; heads-up, the button posts the small blind.
func (s state) blindSeats(button int) (small, big int) {
	small = s.nextAlive(button)
	if s.aliveCount() == minPlayers {
		small = button
	}
	return small, s.nextAlive(small)
}

// dealCards deals one card at a time in order, twice round, then the board.
func dealCards(deck Deck, order []int) (holes [maxPlayers][holeCardsPerSeat]Card, board [boardSize]Card) {
	next := 0
	for round := range holeCardsPerSeat {
		for _, i := range order {
			holes[i][round] = deck[next]
			next++
		}
	}
	copy(board[:], deck[next:])
	return holes, board
}

func (s state) dealStreet(by game.Seat) (state, game.Event) {
	from := s.hand.dealt
	var street Street
	switch from {
	case 0:
		street, s.hand.dealt = StreetFlop, 3
	case 3:
		street, s.hand.dealt = StreetTurn, 4
	default:
		street, s.hand.dealt = StreetRiver, boardSize
	}
	return s, BoardEvent{By: by, Street: street, Cards: slices.Clone(s.hand.board[from:s.hand.dealt])}
}

// advance runs the table until a seat must act or the game is over: it opens
// streets, ends hands and deals new ones as the betting allows.
func (s state) advance(by game.Seat, reason game.ResultReason) (state, []game.Event) {
	var events []game.Event
	for !s.finished {
		chips := s.hand.chips
		if chips.contestants() > 1 {
			if next, ok := chips.nextToAct(s.hand.toAct); ok {
				s.hand.toAct = next
				return s, events
			}
			if chips.bettors() > 1 && s.hand.dealt < boardSize {
				var dealt game.Event
				s, dealt = s.dealStreet(by)
				s.hand.chips = s.hand.chips.newStreet()
				s.hand.toAct = (s.hand.button + 1) % s.n
				events = append(events, dealt)
				continue
			}
		}
		var ended []game.Event
		s, ended = s.endHand(by, reason)
		events = append(events, ended...)
	}
	return s, events
}

// endHand runs out the board when hands are still contested, shows them,
// pays the pots, puts busted seats out and deals the next hand, unless one
// seat is left and the game is over with reason.
func (s state) endHand(by game.Seat, reason game.ResultReason) (state, []game.Event) {
	var events []game.Event
	var values [maxPlayers]HandValue
	shown := []ShownHand{}
	if s.hand.chips.contestants() > 1 {
		for s.hand.dealt < boardSize {
			var dealt game.Event
			s, dealt = s.dealStreet(by)
			events = append(events, dealt)
		}
		values = s.handValues()
		shown = s.showdown(values)
		events = append(events, ShownEvent{By: by, Hands: cloneShown(shown)})
	}
	s, paid, awards := s.settle(by, values)
	events = append(events, paid...)
	s.ended = endedHand{hand: s.hand, shown: shown, awards: awards}
	for i := range s.n {
		if !s.out[i] && s.hand.chips.stack[i] == 0 {
			s.out[i] = true
			events = append(events, BustedEvent{By: by, Seat: seatName(i)})
		}
	}
	if s.aliveCount() == 1 {
		s.finished = true
		s.result = game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{seatName(s.nextAlive(0))}, Reason: reason}
		return s, events
	}
	s, dealt := s.dealHand(by, s.hand.number+1, s.nextAlive(s.hand.button))
	return s, append(events, dealt...)
}

// showdown reveals every contested hand, from the left of the button.
func (s state) showdown(values [maxPlayers]HandValue) []ShownHand {
	var shown []ShownHand
	for _, i := range s.fromLeftOfButton() {
		if s.hand.chips.contesting[i] {
			shown = append(shown, ShownHand{
				Seat:     seatName(i),
				Cards:    slices.Clone(s.hand.holes[i][:]),
				Category: values[i].Category,
			})
		}
	}
	return shown
}

// handValues scores every contested hand on the full board.
func (s state) handValues() [maxPlayers]HandValue {
	var values [maxPlayers]HandValue
	for i := range s.n {
		if s.hand.chips.contesting[i] {
			var seven [sevenSize]Card
			copy(seven[:], s.hand.holes[i][:])
			copy(seven[holeCardsPerSeat:], s.hand.board[:])
			values[i] = bestHand(seven)
		}
	}
	return values
}

// settle returns the uncalled bet and pays the pots from the last side pot to
// the main pot. A seat that has left gets nothing back: every chip it put in
// stays in the pots for the seats still contesting them.
func (s state) settle(by game.Seat, values [maxPlayers]HandValue) (state, []game.Event, []Award) {
	chips := s.hand.chips
	contributions := chips.committed
	var events []game.Event
	if seat, back := uncalled(contributions[:s.n]); back > 0 && !s.out[seat] {
		contributions[seat] -= back
		chips.stack[seat] += back
		events = append(events, ReturnedEvent{By: by, Seat: seatName(seat), Amount: back})
	}
	pots := layerPots(contributions[:s.n], chips.contesting[:s.n])
	awards := []Award{}
	for k := len(pots) - 1; k >= 0; k-- {
		winners := s.potWinners(pots[k].eligible, values)
		for j, amount := range shares(pots[k].amount, len(winners)) {
			chips.stack[winners[j]] += amount
			award := Award{Seat: seatName(winners[j]), Amount: amount, Pot: k}
			awards = append(awards, award)
			events = append(events, WonEvent{By: by, Seat: award.Seat, Amount: amount, Pot: k})
		}
	}
	chips.bet, chips.committed = [maxPlayers]Chips{}, [maxPlayers]Chips{}
	s.hand.chips = chips
	return s, events, awards
}

// potWinners are the eligible seats with the best hand, in odd-chip order:
// clockwise from the left of the button.
func (s state) potWinners(eligible []int, values [maxPlayers]HandValue) []int {
	order := slices.SortedFunc(slices.Values(eligible), func(a, b int) int {
		return cmp.Compare(s.leftOfButtonDistance(a), s.leftOfButtonDistance(b))
	})
	if len(order) == 1 {
		return order
	}
	best := values[slices.MaxFunc(order, func(a, b int) int { return values[a].Compare(values[b]) })]
	return slices.DeleteFunc(order, func(i int) bool { return values[i].Compare(best) != 0 })
}

func (s state) leftOfButtonDistance(i int) int { return (i - s.hand.button - 1 + s.n) % s.n }

func (s state) fromLeftOfButton() []int {
	order := make([]int, s.n)
	for k := range order {
		order[k] = (s.hand.button + 1 + k) % s.n
	}
	return order
}
