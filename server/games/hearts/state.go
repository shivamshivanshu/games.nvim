package hearts

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
)

// state is the match. Every slice is replaced, never written through, so a
// copy of the struct is an independent state.
type state struct {
	order   []game.Seat
	subseed uint64
	handNo  int
	// scores are the match totals from the hands before this one.
	scores []int
	hands  [][]cards.Card
	phase  Phase
	// passes are the cards each seat chose to pass this hand, nil until it
	// chose; received are the cards passed to it.
	passes    [][]cards.Card
	received  [][]cards.Card
	trick     []Play
	turn      int
	trickNo   int
	taken     []int
	broken    bool
	lastTrick *TrickResult
	lastHand  *HandScore
	out       []bool
	game.Ending
}

// stepped is a state after a step, with the events it made.
type stepped struct {
	state
	events []game.Event
}

func (s stepped) with(events ...game.Event) stepped {
	s.events = append(slices.Clone(s.events), events...)
	return s
}

// opened is the state New hands over, with the start of hand 1.
type opened struct {
	state
	start []game.Event
}

// opened is s as New hands it over, starting its hand.
func (s state) opened() opened {
	return opened{state: s, start: []game.Event{HandStartedEvent{By: s.order[0], HandNo: s.handNo, Pass: passOf(s.handNo)}}}
}

// OpeningEvents implements game.Opener.
func (o opened) OpeningEvents() []game.Event { return o.start }

func newState(order []game.Seat, subseed uint64) state {
	s := state{order: order, subseed: subseed, scores: make([]int, players), out: make([]bool, players)}
	return s.deal(1)
}

func passOf(handNo int) PassDirection {
	return [...]PassDirection{PassLeft, PassRight, PassAcross, PassHold}[(handNo-1)%4]
}

// passOffset is how many seats on from the passer this hand's passes go.
func (s state) passOffset() int {
	return map[PassDirection]int{PassLeft: 1, PassAcross: 2, PassRight: 3}[passOf(s.handNo)]
}

// receiver is who seat i passes to this hand.
func (s state) receiver(i int) int { return (i + s.passOffset()) % players }

// giver is who passes to seat i this hand.
func (s state) giver(i int) int { return (i + players - s.passOffset()) % players }

// deal shuffles hand handNo's deck from the sub-seed and deals it one card
// at a time round the table.
func (s state) deal(handNo int) state {
	deck := cards.Shuffled(cards.French(), s.subseed, handNo)
	s.handNo, s.trick, s.trickNo, s.broken = handNo, []Play{}, 1, false
	s.hands = make([][]cards.Card, players)
	for k, c := range deck {
		s.hands[k%players] = append(s.hands[k%players], c)
	}
	for i := range s.hands {
		s.hands[i] = sorted(s.hands[i])
	}
	s.passes, s.received, s.taken = make([][]cards.Card, players), make([][]cards.Card, players), make([]int, players)
	s.phase = PhasePass
	if passOf(handNo) == PassHold {
		s.phase, s.turn = PhasePlay, s.holderOf(twoOfClubs)
	}
	return s
}

func (s state) holderOf(c cards.Card) int {
	return slices.IndexFunc(s.hands, func(hand []cards.Card) bool { return slices.Contains(hand, c) })
}

func (s state) activeCount() int {
	n := 0
	for _, out := range s.out {
		if !out {
			n++
		}
	}
	return n
}

// legal are the cards seat i may play to the trick now.
func (s state) legal(i int) []cards.Card {
	hand := s.hands[i]
	keep := func(ok func(cards.Card) bool) []cards.Card {
		return slices.DeleteFunc(slices.Clone(hand), func(c cards.Card) bool { return !ok(c) })
	}
	if len(s.trick) == 0 {
		if s.trickNo == 1 {
			return keep(func(c cards.Card) bool { return c == twoOfClubs })
		}
		if notHearts := keep(func(c cards.Card) bool { return c.Suit != cards.Hearts }); !s.broken && len(notHearts) > 0 {
			return notHearts
		}
		return slices.Clone(hand)
	}
	if following := keep(func(c cards.Card) bool { return c.Suit == s.trick[0].Card.Suit }); len(following) > 0 {
		return following
	}
	if safe := keep(func(c cards.Card) bool { return points(c) == 0 }); s.trickNo == 1 && len(safe) > 0 {
		return safe
	}
	return slices.Clone(hand)
}

// whyIllegal explains why seat i may not play c, which it holds.
func (s state) whyIllegal(i int, c cards.Card) string {
	switch {
	case len(s.trick) == 0 && s.trickNo == 1:
		return fmt.Sprintf("the first trick is led with the %s", twoOfClubs)
	case len(s.trick) == 0:
		return "hearts are not broken: lead another suit"
	case slices.ContainsFunc(s.hands[i], func(h cards.Card) bool { return h.Suit == s.trick[0].Card.Suit }):
		return fmt.Sprintf("you must follow %s", suitName(s.trick[0].Card.Suit))
	}
	return fmt.Sprintf("no points on the first trick: play a card that is no heart and not the %s", queenOfSpades)
}

// Apply takes a seat's pass, or the card of the seat to play, then plays
// on for seats that left.
func (s state) Apply(seat game.Seat, a Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	i := slices.Index(s.order, seat)
	var (
		next stepped
		err  error
	)
	switch {
	case a.Kind == ActionPass && s.phase == PhasePass:
		next, err = s.choosePass(seat, i, a.Cards)
	case a.Kind == ActionPlay && s.phase == PhasePlay:
		next, err = s.playCard(seat, i, a.Card)
	case a.Kind == ActionPass:
		err = game.Illegalf("you cannot pass now: play a card to the trick")
	case a.Kind == ActionPlay:
		err = game.Illegalf("you cannot play now: choose three cards to pass %s", passOf(s.handNo))
	default:
		err = game.Illegalf("unknown action %q: want %q or %q", a.Kind, ActionPass, ActionPlay)
	}
	if err != nil {
		return nil, nil, err
	}
	return next.state, next.events, nil
}

func (s state) choosePass(seat game.Seat, i int, text string) (stepped, error) {
	pass, err := parsePass(text)
	if err != nil {
		return stepped{}, err
	}
	for _, c := range pass {
		if !slices.Contains(s.hands[i], c) {
			return stepped{}, game.Illegalf("you hold no %s", c)
		}
	}
	chosen := s.withPass(seat, i, sorted(pass))
	return chosen.state.advance(seat).withEarlier(chosen.events), nil
}

func (s state) withPass(by game.Seat, i int, pass []cards.Card) stepped {
	s.passes = slices.Clone(s.passes)
	s.passes[i] = pass
	return stepped{state: s, events: []game.Event{PassChosenEvent{By: by, Seat: s.order[i]}}}
}

func (s state) playCard(seat game.Seat, i int, text string) (stepped, error) {
	c, err := parsePlay(text)
	if err != nil {
		return stepped{}, err
	}
	if !slices.Contains(s.hands[i], c) {
		return stepped{}, game.Illegalf("you hold no %s", c)
	}
	if !slices.Contains(s.legal(i), c) {
		return stepped{}, game.IllegalActionError{Msg: s.whyIllegal(i, c)}
	}
	played := s.play(seat, i, c)
	return played.state.advance(seat).withEarlier(played.events), nil
}

func (s stepped) withEarlier(events []game.Event) stepped {
	s.events = append(slices.Clone(events), s.events...)
	return s
}

// play puts seat i's card c on the trick, settles the trick once all four
// have played, and the hand after its last trick.
func (s state) play(by game.Seat, i int, c cards.Card) stepped {
	s.hands = slices.Clone(s.hands)
	s.hands[i] = slices.DeleteFunc(slices.Clone(s.hands[i]), func(h cards.Card) bool { return h == c })
	s.trick = append(slices.Clone(s.trick), Play{Seat: s.order[i], Card: c})
	s.broken = s.broken || c.Suit == cards.Hearts
	played := stepped{state: s, events: []game.Event{PlayedEvent{By: by, Seat: s.order[i], Card: c}}}
	if len(s.trick) < players {
		played.turn = (i + 1) % players
		return played
	}
	return played.takeTrick(by)
}

// takeTrick gives the trick to the highest card of the suit led, who leads next.
func (s stepped) takeTrick(by game.Seat) stepped {
	led := s.trick[0].Card.Suit
	best, taken := 0, 0
	for k, p := range s.trick {
		taken += points(p.Card)
		if p.Card.Suit == led && p.Card.Rank > s.trick[best].Card.Rank {
			best = k
		}
	}
	winner := slices.Index(s.order, s.trick[best].Seat)
	s.taken = slices.Clone(s.taken)
	s.taken[winner] += taken
	s.lastTrick = &TrickResult{Plays: s.trick, Winner: s.order[winner], Points: taken}
	s.trick, s.turn = []Play{}, winner
	s.trickNo++
	s = s.with(TrickWonEvent{By: by, Seat: s.order[winner], Points: taken})
	if s.trickNo <= handSize {
		return s
	}
	return s.scoreHand(by)
}

// scoreHand adds the hand's points to the scores, a seat that took them all
// shooting the moon, and ends the match or deals the next hand.
func (s stepped) scoreHand(by game.Seat) stepped {
	score := HandScore{HandNo: s.handNo, Points: make([]SeatPoints, players)}
	moon := slices.Index(s.taken, moonPoints)
	if moon >= 0 {
		shooter := s.order[moon]
		score.Moon = &shooter
	}
	s.scores = slices.Clone(s.scores)
	for i, taken := range s.taken {
		if moon >= 0 {
			taken = moonPoints
			if i == moon {
				taken = 0
			}
		}
		score.Points[i] = SeatPoints{Seat: s.order[i], Points: taken}
		s.scores[i] += taken
	}
	s.lastHand = &score
	s = s.with(HandScoredEvent{By: by, Score: score})
	if slices.Max(s.scores) >= ScoreLimit {
		s.state = s.finish(s.finalResult())
		return s
	}
	s.state = s.deal(s.handNo + 1)
	return s.with(HandStartedEvent{By: by, HandNo: s.handNo, Pass: passOf(s.handNo)})
}

// finalResult is a win for the lowest score among the seats still playing,
// shared by a tie, or a draw when every seat ties.
func (s state) finalResult() game.Result {
	winners := s.lowest()
	if len(winners) == players {
		return game.DrawBy(ReasonScoreLimit)
	}
	return game.WinFor(ReasonScoreLimit, winners...)
}

// lowest are the seats still in the game with the lowest score.
func (s state) lowest() []game.Seat {
	best := -1
	var seats []game.Seat
	for i, score := range s.scores {
		switch {
		case s.out[i]:
		case best < 0 || score < best:
			best, seats = score, []game.Seat{s.order[i]}
		case score == best:
			seats = append(seats, s.order[i])
		}
	}
	return seats
}

func (s state) finish(result game.Result) state {
	s.Ending = s.Concluded(result)
	return s
}

// advance plays for the seats that left: each passes its three highest
// cards and plays the first of its legal cards, until a seat still in the
// game is to act. Once every pass is chosen, the cards change hands.
func (s state) advance(by game.Seat) stepped {
	next := stepped{state: s}
	for !next.Over() {
		switch {
		case next.phase == PhasePass:
			for i, out := range next.out {
				if out && next.passes[i] == nil {
					chosen := next.withPass(by, i, highest(next.hands[i]))
					next = stepped{state: chosen.state, events: append(next.events, chosen.events...)}
				}
			}
			if slices.ContainsFunc(next.passes, func(pass []cards.Card) bool { return pass == nil }) {
				return next
			}
			next = stepped{state: next.exchange(), events: next.events}.with(PassedEvent{By: by, Pass: passOf(next.handNo)})
		case next.out[next.turn]:
			played := next.play(by, next.turn, next.legal(next.turn)[0])
			next = played.withEarlier(next.events)
		default:
			return next
		}
	}
	return next
}

func highest(hand []cards.Card) []cards.Card {
	byRank := slices.Clone(hand)
	slices.SortStableFunc(byRank, func(a, b cards.Card) int { return int(b.Rank) - int(a.Rank) })
	return sorted(byRank[:passSize])
}

// exchange hands each seat's pass to its receiver; the holder of the two of
// clubs then leads.
func (s state) exchange() state {
	hands := make([][]cards.Card, players)
	for i, hand := range s.hands {
		hands[i] = slices.DeleteFunc(slices.Clone(hand), func(c cards.Card) bool { return slices.Contains(s.passes[i], c) })
	}
	s.received = make([][]cards.Card, players)
	for i, pass := range s.passes {
		j := s.receiver(i)
		hands[j] = sorted(append(hands[j], pass...))
		s.received[j] = pass
	}
	s.hands, s.phase = hands, PhasePlay
	s.turn = s.holderOf(twoOfClubs)
	return s
}

// ToAct are the seats still to choose a pass, or the seat to play.
func (s state) ToAct() []game.Seat {
	switch {
	case s.Over():
		return nil
	case s.phase == PhasePass:
		var seats []game.Seat
		for i, pass := range s.passes {
			if pass == nil && !s.out[i] {
				seats = append(seats, s.order[i])
			}
		}
		return seats
	}
	return []game.Seat{s.order[s.turn]}
}

// Conclude takes a result the core imposed, such as an abort, so that no seat is left to act.
func (s state) Conclude(result game.Result) game.State[Action, View] { return s.finish(result) }

// Eliminate has the seat's cards played for it from now on, and it cannot
// win. The last seat still in the game wins.
func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	i := slices.Index(s.order, seat)
	if s.Over() || i < 0 || s.out[i] {
		return s, nil
	}
	s.out = slices.Clone(s.out)
	s.out[i] = true
	if s.activeCount() == 1 {
		winner := s.order[slices.Index(s.out, false)]
		return s.finish(game.WinFor(reason, winner)), []game.Event{LeftEvent{By: seat}}
	}
	next := s.advance(seat).withEarlier([]game.Event{LeftEvent{By: seat}})
	return next.state, next.events
}

// View shows seat its own hand and passes, and the tricks.
func (s state) View(seat game.Seat) View {
	v := View{
		HandNo:       s.handNo,
		Pass:         passOf(s.handNo),
		Phase:        s.phase,
		ScoreLimit:   ScoreLimit,
		Seats:        make([]SeatView, players),
		Trick:        slices.Clone(s.trick),
		LastTrick:    s.lastTrick.cloned(),
		HeartsBroken: s.broken,
		MyHand:       []cards.Card{},
		Legal:        []cards.Card{},
		Passed:       []cards.Card{},
		Received:     []cards.Card{},
		LastHand:     s.lastHand.cloned(),
	}
	for i, at := range s.order {
		v.Seats[i] = SeatView{Seat: at, Score: s.scores[i], Taken: s.taken[i], PassChosen: s.passes[i] != nil, Out: s.out[i]}
	}
	i := slices.Index(s.order, seat)
	if i < 0 {
		return v
	}
	v.MyHand = append(v.MyHand, s.hands[i]...)
	if passOf(s.handNo) != PassHold {
		to, from := s.order[s.receiver(i)], s.order[s.giver(i)]
		v.PassTo, v.ReceivedFrom = &to, &from
	}
	v.Passed = append(v.Passed, s.passes[i]...)
	v.Received = append(v.Received, s.received[i]...)
	v.Prompt = s.prompt(i)
	switch v.Prompt {
	case PromptPass:
		v.MustPass = passSize
	case PromptLead, PromptPlay:
		v.Legal = s.legal(i)
	}
	return v
}

func (s state) prompt(i int) Prompt {
	switch {
	case s.Over():
		return PromptNone
	case s.phase == PhasePass && s.passes[i] == nil:
		return PromptPass
	case s.phase == PhasePass:
		return PromptWait
	case i == s.turn && len(s.trick) == 0:
		return PromptLead
	case i == s.turn:
		return PromptPlay
	}
	return PromptNone
}

func (t *TrickResult) cloned() *TrickResult {
	if t == nil {
		return nil
	}
	c := *t
	c.Plays = slices.Clone(t.Plays)
	return &c
}

func (h *HandScore) cloned() *HandScore {
	if h == nil {
		return nil
	}
	c := *h
	c.Points = slices.Clone(h.Points)
	if h.Moon != nil {
		moon := *h.Moon
		c.Moon = &moon
	}
	return &c
}
