package yahtzee

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/dice"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

type state struct {
	order     []game.Seat
	cards     []card
	turn      int
	dice      hand
	held      [DiceCount]bool
	rollsLeft int
	result    game.Result
	finished  bool
}

func newState(order []game.Seat) state {
	return state{order: order, cards: make([]card, len(order)), rollsLeft: RollsPerTurn}
}

func (s state) rolled() bool { return s.rollsLeft < RollsPerTurn }

func (s state) Apply(seat game.Seat, action Action, rng *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if !slices.Contains(s.ToAct(), seat) {
		return nil, nil, game.ErrNotYourTurn
	}
	switch action.Kind {
	case ActionRoll:
		return s.applyRoll(rng)
	case ActionHold:
		return s.applyHold(action.Die)
	case ActionScore:
		return s.applyScore(action.Box)
	}
	return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
}

func (s state) applyRoll(rng *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if s.rollsLeft == 0 {
		return nil, nil, game.IllegalActionError{Msg: "no rolls left: score a box"}
	}
	next := s
	var free []int
	for i, held := range s.held {
		if !held {
			free = append(free, i)
		}
	}
	for k, face := range dice.Roll(rng, len(free), dice.Six) {
		next.dice[free[k]] = face
	}
	next.rollsLeft--
	return next, []game.Event{RolledEvent{By: s.order[s.turn], Dice: slices.Clone(next.dice[:])}}, nil
}

func parseDie(text string) (int, error) {
	n, err := strconv.Atoi(text)
	if err != nil || n < 1 || n > DiceCount {
		return 0, game.IllegalActionError{Msg: fmt.Sprintf("die must be 1 to %d, got %q", DiceCount, text)}
	}
	return n - 1, nil
}

func (s state) applyHold(text string) (game.State[Action, View], []game.Event, error) {
	i, err := parseDie(text)
	if err != nil {
		return nil, nil, err
	}
	switch {
	case !s.rolled():
		return nil, nil, game.IllegalActionError{Msg: "roll first"}
	case s.rollsLeft == 0:
		return nil, nil, game.IllegalActionError{Msg: "no rolls left: score a box"}
	}
	next := s
	next.held[i] = !s.held[i]
	return next, nil, nil
}

func parseBox(text string) (Box, error) {
	box := Box(text)
	if _, ok := boxIndex(box); !ok {
		return "", game.IllegalActionError{Msg: fmt.Sprintf("no box %q", text)}
	}
	return box, nil
}

func (s state) applyScore(text string) (game.State[Action, View], []game.Event, error) {
	box, err := parseBox(text)
	if err != nil {
		return nil, nil, err
	}
	if !s.rolled() {
		return nil, nil, game.IllegalActionError{Msg: "roll first"}
	}
	c := s.cards[s.turn]
	if c.isFilled(box) {
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("%s is already scored", box)}
	}
	options := c.options(s.dice)
	at := slices.IndexFunc(options, func(o Option) bool { return o.Box == box })
	if at < 0 {
		boxes := xslices.Map(options, func(o Option) Box { return o.Box })
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("the joker rule allows only %v", boxes)}
	}
	bonus := c.bonusFor(s.dice)
	next := s
	next.cards = slices.Clone(s.cards)
	next.cards[s.turn] = c.withScore(box, options[at].Score, bonus)
	event := ScoredEvent{By: s.order[s.turn], Box: box, Score: options[at].Score, YahtzeeBonus: bonus}
	return next.passTurn(), []game.Event{event}, nil
}

// passTurn gives fresh dice to the next seat still playing, or ends the game
// once every seat still playing has filled its card.
func (s state) passTurn() state {
	s.dice, s.held, s.rollsLeft = hand{}, [DiceCount]bool{}, RollsPerTurn
	for range s.order {
		s.turn = (s.turn + 1) % len(s.order)
		if !s.cards[s.turn].out {
			break
		}
	}
	if s.cards[s.turn].full() {
		return s.finish(s.finalResult())
	}
	return s
}

func (s state) playing() []int {
	var seats []int
	for i, c := range s.cards {
		if !c.out {
			seats = append(seats, i)
		}
	}
	return seats
}

// finalResult ranks the full cards: the highest total wins, and a tie between
// every seat still playing is a draw.
func (s state) finalResult() game.Result {
	playing := s.playing()
	best := slices.Max(xslices.Map(playing, func(i int) int { return s.cards[i].total() }))
	var winners []game.Seat
	for _, i := range playing {
		if s.cards[i].total() == best {
			winners = append(winners, s.order[i])
		}
	}
	if len(playing) > 1 && len(winners) == len(playing) {
		return game.Result{Kind: game.ResultDraw, Reason: ReasonScorecardsFull}
	}
	return game.Result{Kind: game.ResultDecisive, Winners: winners, Reason: ReasonScorecardsFull}
}

func (s state) finish(result game.Result) state {
	s.result, s.finished = result, true
	return s
}

func (s state) View(viewer game.Seat) View {
	toAct := slices.Contains(s.ToAct(), viewer)
	shown := []dice.Face{}
	options := []Option{}
	if s.rolled() {
		shown = slices.Clone(s.dice[:])
		if toAct {
			options = s.cards[s.turn].options(s.dice)
		}
	}
	return View{
		Round:     min(s.cards[s.turn].boxesFilled()+1, Rounds),
		Dice:      shown,
		Held:      slices.Clone(s.held[:]),
		RollsLeft: s.rollsLeft,
		Cards:     s.cardViews(),
		CanRoll:   toAct && s.rollsLeft > 0,
		CanHold:   toAct && s.rolled() && s.rollsLeft > 0,
		Options:   options,
	}
}

func (s state) cardViews() []Card {
	views := make([]Card, len(s.cards))
	for i, c := range s.cards {
		scores := make(map[Box]int, boxCount)
		for k, box := range Boxes() {
			if c.filled[k] {
				scores[box] = c.points[k]
			}
		}
		views[i] = Card{
			Seat: s.order[i], Scores: scores,
			UpperTotal: c.upperTotal(), UpperBonus: c.upperBonus(), YahtzeeBonus: c.yahtzeeBonus,
			Total: c.total(), Out: c.out,
		}
	}
	return views
}

func (s state) ToAct() []game.Seat {
	if s.finished {
		return nil
	}
	return []game.Seat{s.order[s.turn]}
}

func (s state) Outcome() (game.Result, bool) { return s.result, s.finished }

// Scores are each seat's card total, so the totals rank the seats behind the winners.
func (s state) Scores() map[game.Seat]int64 {
	scores := make(map[game.Seat]int64, len(s.order))
	for i, seat := range s.order {
		scores[seat] = int64(s.cards[i].total())
	}
	return scores
}

// Conclude takes a result the core imposed, such as an abort.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	if s.finished {
		return s
	}
	return s.finish(result)
}

// Eliminate takes the seat out with its card as it stands. A solo game is
// lost; with others, the last seat still playing wins, and otherwise play goes on.
func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	i := slices.Index(s.order, seat)
	if s.finished || i < 0 || s.cards[i].out {
		return s, nil
	}
	playing := xslices.Map(s.playing(), func(i int) game.Seat { return s.order[i] })
	next := s
	next.cards = slices.Clone(s.cards)
	next.cards[i].out = true
	if len(playing) <= 2 {
		return next.finish(game.ForfeitToRemaining(playing, seat, reason)), nil
	}
	if s.turn == i {
		return next.passTurn(), nil
	}
	return next, nil
}
