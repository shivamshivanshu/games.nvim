package gametest

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

// flaw is the conformance defect a tableRules game deliberately has.
type flaw int

const (
	honest flaw = iota
	leakPrivateInView
	leakPrivateInViewKey
	leakPrivateInEvent
	leakPrivateInEventKind
	leakNeighbourInEvent
	leakNeighbourInOpening
	finishOnNeighbourCut
	leakSecretInView
	leakEscapedSecretInView
	leakSecretInEvent
	finishOnFirstElimination
	ignoreElimination
	eliminateRewritesFinished
	eliminateRewritesPlayedResult
	concludeKeepsPlaying
	uncataloguedEvent
	clockWithoutTimeControl
	viewSortsDeck
)

const (
	tableSecret   = "answer-42"
	escapedSecret = `a<b&"c`
)

// handOf is seat's private hand in a round; every step deals a new round.
func handOf(seat game.Seat, round int) string { return fmt.Sprintf("hand-%s-r%d", seat, round) }

// hand is seat's hand now; each cut of seat's share deals it a later round's.
func (s tableState) hand(seat game.Seat) string { return handOf(seat, s.round+s.cuts[seat]) }

// tableRules seats everyone at a table where each holds a private hand, re-dealt every step,
// and the game holds a secret that is revealed once it finishes.
type tableRules struct {
	max  int
	flaw flaw
}

type tableOpts struct{}

const tableSay game.ActionKind = "say"

// tableAction's Aside is said along with the hand, but never sent.
type tableAction struct {
	Kind   game.ActionKind `json:"kind"`
	Finish bool
	Aside  string `json:"-"`
}

type tableView struct {
	Hand   string
	Others []string
	Counts map[string]int
	Answer string
}

type saidEvent struct {
	By   game.Seat `json:"by"`
	Text string    `json:"text"`
}

func (saidEvent) EventKind() game.EventKind { return "said" }

type namedEvent struct {
	Kind string `json:"-"`
}

func (e namedEvent) EventKind() game.EventKind { return game.EventKind(e.Kind) }

func (r tableRules) Info() game.Info {
	info := game.Info{
		Type:    "table",
		Players: game.PlayerRange{Min: 1, Max: r.max},
		Actions: []game.ActionSpec{{Kind: tableSay, Description: "Show the hand."}},
		Events:  []game.Event{saidEvent{}},
	}
	if r.flaw == clockWithoutTimeControl {
		info.Capabilities = []game.Capability{game.CapClock}
	}
	return info
}

func (r tableRules) New(_ tableOpts, challenger game.UserID, invitees []game.UserID, _ *rand.Rand) (game.State[tableAction, tableView], map[game.UserID]game.Seat, error) {
	users := append([]game.UserID{challenger}, invitees...)
	seats := make(map[game.UserID]game.Seat, len(users))
	all := make([]game.Seat, len(users))
	for i, u := range users {
		all[i] = game.Seat(fmt.Sprintf("s%d", i+1))
		seats[u] = all[i]
	}
	return tableState{flaw: r.flaw, seats: all, alive: all, cuts: map[game.Seat]int{}, deck: []string{"c", "a", "b"}}, seats, nil
}

type tableState struct {
	game.Ending
	flaw      flaw
	seats     []game.Seat
	alive     []game.Seat
	round     int
	cuts      map[game.Seat]int
	deck      []string
	playedOut bool
}

func privateHands(s tableState, seat game.Seat) []string { return []string{s.hand(seat)} }

// cutHand deals owner another hand, a change only owner may see.
func cutHand(st game.State[tableAction, tableView], owner game.Seat) (game.State[tableAction, tableView], []game.Seat) {
	s := st.(tableState)
	s.cuts = maps.Clone(s.cuts)
	s.cuts[owner]++
	return s, []game.Seat{owner}
}

func (s tableState) Apply(seat game.Seat, a tableAction, _ *rand.Rand) (game.State[tableAction, tableView], []game.Event, error) {
	said := s.showdown(seat)
	said.Text += a.Aside
	if s.flaw == viewSortsDeck {
		said.Text += " drew " + s.deck[0]
		s.deck = s.deck[1:]
	}
	events := []game.Event{said}
	s.round++
	switch s.flaw {
	case leakPrivateInEventKind:
		events = append(events, namedEvent{Kind: handOf(seat, s.round)})
	case uncataloguedEvent:
		events = append(events, namedEvent{Kind: "shrugged"})
	}
	if a.Finish || (s.flaw == finishOnNeighbourCut && s.cuts[s.neighbour(seat)] > 0) {
		s.Ending, s.playedOut = s.Concluded(game.Result{Kind: game.ResultDraw}), true
	}
	return s, events, nil
}

// showdown reveals seat's hand of the round that is ending, which is no longer private.
func (s tableState) showdown(seat game.Seat) saidEvent {
	switch s.flaw {
	case leakPrivateInEvent:
		return saidEvent{By: seat, Text: handOf(seat, s.round+1)}
	case leakSecretInEvent:
		return saidEvent{By: seat, Text: tableSecret}
	case leakNeighbourInEvent:
		return saidEvent{By: seat, Text: s.hand(s.neighbour(seat))}
	}
	return saidEvent{By: seat, Text: s.hand(seat)}
}

func (s tableState) neighbour(seat game.Seat) game.Seat {
	return s.seats[(slices.Index(s.seats, seat)+1)%len(s.seats)]
}

func (s tableState) OpeningEvents() []game.Event {
	if s.flaw != leakNeighbourInOpening {
		return nil
	}
	return []game.Event{saidEvent{By: s.seats[0], Text: s.hand(s.neighbour(s.seats[0]))}}
}

func (s tableState) View(seat game.Seat) tableView {
	if s.flaw == viewSortsDeck {
		slices.Sort(s.deck)
	}
	v := tableView{Hand: s.hand(seat)}
	switch {
	case s.Over() || s.flaw == leakSecretInView:
		v.Answer = tableSecret
	case s.flaw == leakEscapedSecretInView:
		v.Answer = escapedSecret
	}
	for _, other := range s.seats {
		if other == seat {
			continue
		}
		switch s.flaw {
		case leakPrivateInView:
			v.Others = append(v.Others, s.hand(other))
		case leakPrivateInViewKey:
			v.Counts = map[string]int{s.hand(other): 1}
		}
	}
	return v
}

func (s tableState) ToAct() []game.Seat {
	if s.Over() {
		return nil
	}
	return s.alive
}

func (s tableState) Conclude(r game.Result) game.State[tableAction, tableView] {
	if s.flaw != concludeKeepsPlaying {
		s.Ending = s.Concluded(r)
	}
	return s
}

func (s tableState) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[tableAction, tableView], []game.Event) {
	if s.Over() {
		if s.flaw == eliminateRewritesFinished || (s.flaw == eliminateRewritesPlayedResult && s.playedOut) {
			s.Ending = game.Ending{}.Concluded(game.ForfeitToRemaining(s.seats, seat, reason))
		}
		return s, nil
	}
	if s.flaw == ignoreElimination {
		return s, nil
	}
	s.alive = slices.DeleteFunc(slices.Clone(s.alive), func(a game.Seat) bool { return a == seat })
	if len(s.alive) < 2 || s.flaw == finishOnFirstElimination {
		s.Ending = s.Concluded(game.WinFor(reason, s.alive...))
	}
	return s, nil
}
