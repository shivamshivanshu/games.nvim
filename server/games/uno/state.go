package uno

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

const (
	drawTwoCards     = 2
	drawFourCards    = 4
	failedChallenge  = 6
	clockwiseStep    = 1
	counterclockwise = -1
)

// state is the table. Every slice is replaced, never written through, so a
// copy of the struct is an independent state.
type state struct {
	order []game.Seat
	hands [][]Card
	// pile is the draw pile, top first; discard is the discard pile, top last.
	pile    []Card
	discard []Card
	color   Color
	turn    int
	step    int
	phase   Phase
	// drawn is the card the seat to act just drew, in PhaseDrawn.
	drawn Card
	// offender played the Wild Draw Four facing the seat to act in
	// PhaseChallenge, over the colour before.
	offender int
	before   Color
	out      []bool
	subseed  uint64
	shuffles int
	game.Ending
}

// opened is the state New hands over, with the events of the deal.
type opened struct {
	state
	deal []game.Event
}

// OpeningEvents implements game.Opener.
func (o opened) OpeningEvents() []game.Event { return o.deal }

// deal shuffles the deck and deals it.
func deal(order []game.Seat, dealer int, subseed uint64) (state, []game.Event) {
	return dealFrom(order, dealer, subseed, cards.Shuffled(orderedDeck(), subseed, 0))
}

// dealFrom deals handSize cards to each seat in turn from the dealer's left
// and turns up the next card, putting back a Wild Draw Four and shuffling
// again until another card shows. The first card's effect then applies to the
// seat left of the dealer, as if the dealer had played it.
func dealFrom(order []game.Seat, dealer int, subseed uint64, pile []Card) (state, []game.Event) {
	s := state{
		order:   order,
		hands:   make([][]Card, len(order)),
		step:    clockwiseStep,
		out:     make([]bool, len(order)),
		subseed: subseed,
		phase:   PhasePlay,
	}
	for k := range handSize * len(order) {
		i := (dealer + 1 + k) % len(order)
		s.hands[i] = append(s.hands[i], pile[k])
	}
	pile = pile[handSize*len(order):]
	for pile[0].Face == WildDrawFour {
		s.shuffles++
		pile = cards.Shuffled(pile, subseed, s.shuffles)
	}
	s.pile, s.discard = pile[1:], []Card{pile[0]}
	top := pile[0]
	s.color = top.Color
	by := order[dealer]
	events := []game.Event{StartedEvent{By: by, Dealer: by, Top: top}}
	s.turn = dealer
	switch top.Face {
	case Wild:
		s.turn, s.phase = s.next(dealer), PhaseColor
		return s, events
	case Reverse:
		s.step = counterclockwise
		return s, append(events, ReversedEvent{By: by, Direction: s.direction()})
	}
	effects := s.afterPlay(by, top)
	return effects.state, append(events, effects.events...)
}

func (s state) direction() Direction {
	if s.step == clockwiseStep {
		return Clockwise
	}
	return CounterClockwise
}

func (s state) activeCount() int { return xslices.Count(s.out, false) }

// next is the first seat after i, in the direction of play, still in the game.
func (s state) next(i int) int {
	n := len(s.order)
	for k := 1; k <= n; k++ {
		if j := ((i+k*s.step)%n + n) % n; !s.out[j] {
			return j
		}
	}
	return i
}

func (s state) top() Card { return s.discard[len(s.discard)-1] }

// matches is whether c may go on the discard pile now.
func (s state) matches(c Card) bool {
	return c.Face.wild() || c.Color == s.color || c.Face == s.top().Face
}

// stepped is a state after a step, with the events it made.
type stepped struct {
	state
	events []game.Event
}

func (s stepped) then(next stepped) stepped {
	next.events = append(slices.Clone(s.events), next.events...)
	return next
}

// afterPlay applies card's effect, just played by the seat to act's step
// author by, to the seats after the turn, and passes the turn on.
func (s state) afterPlay(by game.Seat, card Card) stepped {
	victim := s.next(s.turn)
	switch card.Face {
	case Skip:
		s.turn = s.next(victim)
		return stepped{s, []game.Event{SkippedEvent{By: by, Seat: s.order[victim]}}}
	case Reverse:
		s.step = -s.step
		events := []game.Event{ReversedEvent{By: by, Direction: s.direction()}}
		if s.activeCount() == minPlayers {
			s.turn = s.next(victim)
			return stepped{s, append(events, SkippedEvent{By: by, Seat: s.order[victim]})}
		}
		s.turn = s.next(s.turn)
		return stepped{s, events}
	case DrawTwo:
		drew := s.draw(by, victim, drawTwoCards)
		drew.turn = drew.next(victim)
		return drew.then(stepped{drew.state, []game.Event{SkippedEvent{By: by, Seat: s.order[victim]}}})
	}
	s.turn = victim
	return stepped{s, nil}
}

// draw has seat i take up to n cards from the draw pile, turning the
// discard pile under its top card into a new one when the pile runs out.
// It takes fewer only when both are spent.
func (s state) draw(by game.Seat, i, n int) stepped {
	var events []game.Event
	taken := 0
	hand := slices.Clone(s.hands[i])
	for taken < n {
		if len(s.pile) == 0 {
			if len(s.discard) == 1 {
				break
			}
			s.shuffles++
			s.pile = cards.Shuffled(s.discard[:len(s.discard)-1], s.subseed, s.shuffles)
			s.discard = []Card{s.top()}
			events = append(events, ReshuffledEvent{By: by, Cards: len(s.pile)})
		}
		hand = append(hand, s.pile[0])
		s.pile = s.pile[1:]
		taken++
	}
	s.hands = slices.Clone(s.hands)
	s.hands[i] = hand
	return stepped{s, append(events, DrewEvent{By: by, Seat: s.order[i], Count: taken})}
}

// Apply plays the move of the seat to act.
func (s state) Apply(seat game.Seat, a Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if !slices.Contains(s.phaseKinds(), a.Kind) {
		return nil, nil, game.IllegalActionError{Msg: s.phaseHint(a.Kind)}
	}
	var (
		next stepped
		err  error
	)
	switch a.Kind {
	case ActionPlay:
		next, err = s.applyPlay(seat, a.Card)
	case ActionDraw:
		next = s.applyDraw(seat)
	case ActionPass:
		s.phase, s.turn = PhasePlay, s.next(s.turn)
		next = stepped{s, []game.Event{PassedEvent{By: seat}}}
	case ActionChallenge:
		next = s.applyChallenge(seat)
	case ActionAccept:
		drew := s.draw(seat, s.turn, drawFourCards)
		drew.phase, drew.turn = PhasePlay, drew.next(s.turn)
		next = drew
	case ActionColor:
		next, err = s.applyColor(seat, a.Color)
	}
	if err != nil {
		return nil, nil, err
	}
	return next.state, next.events, nil
}

func (s state) phaseHint(kind game.ActionKind) string {
	switch s.phase {
	case PhaseColor:
		return fmt.Sprintf("you cannot %s now: name the colour for the Wild turned up", kind)
	case PhaseDrawn:
		return fmt.Sprintf("you cannot %s now: play the card you drew or pass", kind)
	case PhaseChallenge:
		return fmt.Sprintf("you cannot %s now: challenge the Wild Draw Four or accept it", kind)
	}
	return fmt.Sprintf("you cannot %s now: play a card or draw", kind)
}

func (s state) applyPlay(seat game.Seat, text string) (stepped, error) {
	p, err := parsePlay(text)
	if err != nil {
		return stepped{}, err
	}
	hand := s.hands[s.turn]
	at := slices.Index(hand, p.card)
	switch {
	case at < 0:
		return stepped{}, game.Illegalf("you hold no %s", p.card)
	case s.phase == PhaseDrawn && p.card != s.drawn:
		return stepped{}, game.Illegalf("after drawing you may play only the card you drew, %s", s.drawn)
	case !s.matches(p.card):
		return stepped{}, game.Illegalf("%s does not match the %s colour or the %s on the pile", p.card, s.color, s.top())
	}
	s.hands = slices.Clone(s.hands)
	s.hands[s.turn] = slices.Delete(slices.Clone(hand), at, at+1)
	s.discard = append(slices.Clone(s.discard), p.card)
	before := s.color
	s.color = p.card.Color
	if p.card.Face.wild() {
		s.color = p.names
	}
	s.phase = PhasePlay
	played := stepped{s, []game.Event{PlayedEvent{By: seat, Card: p.card, Color: s.color}}}
	if len(s.hands[s.turn]) == 0 {
		s.Ending = s.Concluded(game.WinFor(ReasonWentOut, seat))
		return stepped{s, played.events}, nil
	}
	if p.card.Face == WildDrawFour {
		s.offender, s.before = s.turn, before
		s.phase, s.turn = PhaseChallenge, s.next(s.turn)
		return stepped{s, played.events}, nil
	}
	return played.then(s.afterPlay(seat, p.card)), nil
}

// applyDraw draws one card, which the seat then plays, if it matches, or
// keeps. Every draw that takes a card waits on that choice, so that the turn
// shows no other seat whether the card could be played. With nothing left to
// draw the turn passes.
func (s state) applyDraw(seat game.Seat) stepped {
	drew := s.draw(seat, s.turn, 1)
	hand := drew.hands[drew.turn]
	if len(hand) > len(s.hands[s.turn]) {
		drew.phase, drew.drawn = PhaseDrawn, hand[len(hand)-1]
		return drew
	}
	drew.turn = drew.next(drew.turn)
	return drew
}

// applyChallenge checks the offender's hand for a card of the colour in play
// before its Wild Draw Four. A guilty offender draws four and the challenger
// plays on; otherwise the challenger draws six and loses its turn.
func (s state) applyChallenge(seat game.Seat) stepped {
	guilty := slices.ContainsFunc(s.hands[s.offender], func(c Card) bool { return c.Color == s.before })
	challenged := stepped{s, []game.Event{ChallengedEvent{By: seat, Seat: s.order[s.offender], Guilty: guilty}}}
	s.phase = PhasePlay
	if guilty {
		return challenged.then(s.draw(seat, s.offender, drawFourCards))
	}
	drew := s.draw(seat, s.turn, failedChallenge)
	drew.turn = drew.next(s.turn)
	return challenged.then(drew)
}

func (s state) applyColor(seat game.Seat, text string) (stepped, error) {
	color, err := parseColor(text)
	if err != nil {
		return stepped{}, err
	}
	s.color, s.phase = color, PhasePlay
	return stepped{s, []game.Event{ColorEvent{By: seat, Color: color}}}, nil
}

// phaseKinds are the action kinds the phase takes from the seat to act.
func (s state) phaseKinds() []game.ActionKind {
	switch s.phase {
	case PhaseColor:
		return []game.ActionKind{ActionColor}
	case PhaseDrawn:
		return []game.ActionKind{ActionPlay, ActionPass}
	case PhaseChallenge:
		return []game.ActionKind{ActionChallenge, ActionAccept}
	}
	return []game.ActionKind{ActionPlay, ActionDraw}
}

// legal are the phase's action kinds less play when no card can be played.
func (s state) legal() []game.ActionKind {
	kinds := s.phaseKinds()
	if len(s.playable()) == 0 {
		return slices.DeleteFunc(kinds, func(k game.ActionKind) bool { return k == ActionPlay })
	}
	return kinds
}

// playable are the distinct cards of the seat to act's hand that may be played now, sorted.
func (s state) playable() []Card {
	switch s.phase {
	case PhaseDrawn:
		if s.matches(s.drawn) {
			return []Card{s.drawn}
		}
	case PhasePlay:
		matching := slices.DeleteFunc(xslices.Clone(s.hands[s.turn]), func(c Card) bool { return !s.matches(c) })
		slices.SortFunc(matching, cardOrder)
		return slices.Compact(matching)
	}
	return []Card{}
}

// ToAct is the one seat to act, or none once the game is over.
func (s state) ToAct() []game.Seat { return s.ToActOf(s.order[s.turn]) }

// Conclude takes a result the core imposed, such as an abort, so that no seat is left to act.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Ending = s.Concluded(result)
	return s
}

// Eliminate takes the seat's hand out of play. The last seat left wins.
// Otherwise a Wild Draw Four the seat played, or faced, is void, and if the
// seat was to act the turn passes on.
func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	i := slices.Index(s.order, seat)
	if s.Over() || i < 0 || s.out[i] {
		return s, nil
	}
	s.out = slices.Clone(s.out)
	s.out[i] = true
	s.hands = slices.Clone(s.hands)
	s.hands[i] = nil
	events := []game.Event{LeftEvent{By: seat}}
	if s.activeCount() == 1 {
		winner := s.order[slices.Index(s.out, false)]
		s.Ending = s.Concluded(game.WinFor(reason, winner))
		return s, events
	}
	if s.phase == PhaseChallenge && s.offender == i {
		s.phase = PhasePlay
	}
	if s.turn == i {
		s.turn = s.next(i)
		if s.phase != PhaseColor {
			s.phase = PhasePlay
		}
	}
	return s, events
}

// View shows seat its own hand and every other hand as a count.
func (s state) View(seat game.Seat) View {
	v := View{
		Top:       s.top(),
		Color:     s.color,
		Direction: s.direction(),
		Phase:     s.phase,
		DrawPile:  len(s.pile),
		Seats:     make([]SeatView, len(s.order)),
		MyHand:    []Card{},
		Legal:     []game.ActionKind{},
		Playable:  []Card{},
	}
	for i, at := range s.order {
		v.Seats[i] = SeatView{Seat: at, Cards: len(s.hands[i]), Out: s.out[i]}
	}
	i := slices.Index(s.order, seat)
	if i < 0 {
		return v
	}
	v.MyHand = append(v.MyHand, s.hands[i]...)
	slices.SortFunc(v.MyHand, cardOrder)
	if !s.Over() && i == s.turn {
		v.Legal, v.Playable = s.legal(), s.playable()
		v.Prompt = promptFor(s.phase, v.Playable)
	}
	return v
}

func promptFor(phase Phase, playable []Card) Prompt {
	switch phase {
	case PhaseColor:
		return PromptColor
	case PhasePlay:
		if len(playable) == 0 {
			return PromptDraw
		}
		return PromptPlayOrDraw
	case PhaseDrawn:
		if len(playable) == 0 {
			return PromptKeep
		}
		return PromptPlayOrKeep
	case PhaseChallenge:
		return PromptChallenge
	}
	return PromptNone
}
