package gametest

import (
	"fmt"
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
	leakSecretInView
	leakEscapedSecretInView
	leakSecretInEvent
	finishOnFirstElimination
	ignoreElimination
	eliminateRewritesFinished
	eliminateRewritesPlayedResult
	concludeKeepsPlaying
)

const (
	tableSecret   = "answer-42"
	escapedSecret = `a<b&"c`
)

// handOf is seat's private hand in a round; every step deals a new round.
func handOf(seat game.Seat, round int) string { return fmt.Sprintf("hand-%s-r%d", seat, round) }

// tableRules seats everyone at a table where each holds a private hand, re-dealt every step,
// and the game holds a secret that is revealed once it finishes.
type tableRules struct {
	max  int
	flaw flaw
}

type tableOpts struct{}

type tableAction struct{ Finish bool }

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
	return game.Info{Type: "table", Players: game.PlayerRange{Min: 1, Max: r.max}}
}

func (r tableRules) New(_ tableOpts, challenger game.UserID, invitees []game.UserID, _ *rand.Rand) (game.State[tableAction, tableView], map[game.UserID]game.Seat, error) {
	users := append([]game.UserID{challenger}, invitees...)
	seats := make(map[game.UserID]game.Seat, len(users))
	all := make([]game.Seat, len(users))
	for i, u := range users {
		all[i] = game.Seat(fmt.Sprintf("s%d", i+1))
		seats[u] = all[i]
	}
	return tableState{flaw: r.flaw, seats: all, alive: all}, seats, nil
}

type tableState struct {
	game.Ending
	flaw      flaw
	seats     []game.Seat
	alive     []game.Seat
	round     int
	playedOut bool
}

func privateHands(s tableState, seat game.Seat) []string { return []string{handOf(seat, s.round)} }

func (s tableState) Apply(seat game.Seat, a tableAction, _ *rand.Rand) (game.State[tableAction, tableView], []game.Event, error) {
	events := []game.Event{s.showdown(seat)}
	s.round++
	if s.flaw == leakPrivateInEventKind {
		events = append(events, namedEvent{Kind: handOf(seat, s.round)})
	}
	if a.Finish {
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
	}
	return saidEvent{By: seat, Text: handOf(seat, s.round)}
}

func (s tableState) View(seat game.Seat) tableView {
	v := tableView{Hand: handOf(seat, s.round)}
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
			v.Others = append(v.Others, handOf(other, s.round))
		case leakPrivateInViewKey:
			v.Counts = map[string]int{handOf(other, s.round): 1}
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
