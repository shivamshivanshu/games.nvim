package ludo

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const (
	dieFaces     = 6
	bonusRoll    = Roll(dieFaces)
	forfeitSixes = 3
)

type state struct {
	order    []game.Seat
	tokens   map[game.Seat]Tokens
	turn     game.Seat
	phase    Phase
	lastRoll Roll
	sixes    int
	result   game.Result
	finished bool
}

func newState(seats []game.Seat, first game.Seat) state {
	tokens := make(map[game.Seat]Tokens, len(seats))
	for _, seat := range seats {
		tokens[seat] = Tokens{ProgressBase, ProgressBase, ProgressBase, ProgressBase}
	}
	return state{order: clockwise(seats), tokens: tokens, turn: first, phase: PhaseRoll}
}

func rollDie(rng *rand.Rand) Roll { return Roll(rng.IntN(dieFaces) + 1) }

func (s state) Apply(seat game.Seat, action Action, rng *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if !slices.Contains(s.ToAct(), seat) {
		return nil, nil, game.ErrNotYourTurn
	}
	switch action.Kind {
	case ActionRoll:
		return s.applyRoll(rng)
	case ActionMove:
		return s.applyMove(action.Token)
	}
	return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
}

func (s state) applyRoll(rng *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if s.phase != PhaseRoll {
		return nil, nil, game.IllegalActionError{Msg: "move a token first"}
	}
	roll := rollDie(rng)
	next := s
	next.lastRoll = roll
	next.sixes = 0
	if roll == bonusRoll {
		next.sixes = s.sixes + 1
	}
	events := []game.Event{RolledEvent{By: s.turn, Value: roll}}
	switch {
	case next.sixes == forfeitSixes:
		return next.passTurn(), append(events, ForfeitedEvent{By: s.turn}), nil
	case len(next.movable()) == 0:
		return next.afterTurnAction(), append(events, PassedEvent{By: s.turn}), nil
	}
	next.phase = PhaseMove
	return next, events, nil
}

func (s state) applyMove(token Token) (game.State[Action, View], []game.Event, error) {
	if s.phase != PhaseMove {
		return nil, nil, game.IllegalActionError{Msg: "roll first"}
	}
	if token < 1 || token > TokensPerSeat {
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("token must be 1 to %d, got %d", TokensPerSeat, token)}
	}
	if !slices.Contains(s.movable(), token) {
		return nil, nil, game.IllegalActionError{Msg: fmt.Sprintf("token %d cannot move %d", token, s.lastRoll)}
	}
	from := s.tokens[s.turn][token-1]
	to, _ := advance(from, s.lastRoll)
	next := s
	next.tokens = maps.Clone(s.tokens)
	events := []game.Event{MovedEvent{By: s.turn, Token: token, From: from, To: to}}
	if victim, victimToken, ok := s.captureAt(to); ok {
		next.tokens[victim] = withProgress(next.tokens[victim], victimToken, ProgressBase)
		events = append(events, CapturedEvent{By: s.turn, Victim: victim, Token: victimToken})
	}
	next.tokens[s.turn] = withProgress(next.tokens[s.turn], token, to)
	if allHome(next.tokens[s.turn]) {
		return next.finish(game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{s.turn}, Reason: ReasonAllHome}), events, nil
	}
	return next.afterTurnAction(), events, nil
}

// afterTurnAction lets a six roll again and passes the turn otherwise.
func (s state) afterTurnAction() state {
	if s.lastRoll == bonusRoll {
		s.phase = PhaseRoll
		return s
	}
	return s.passTurn()
}

func (s state) finish(result game.Result) state {
	s.result, s.finished, s.phase = result, true, PhaseFinished
	return s
}

func (s state) passTurn() state {
	at := slices.Index(s.order, s.turn)
	s.turn = s.order[(at+1)%len(s.order)]
	s.phase = PhaseRoll
	s.sixes = 0
	return s
}

func (s state) movable() []Token {
	movable := []Token{}
	for token := Token(1); token <= TokensPerSeat; token++ {
		to, ok := advance(s.tokens[s.turn][token-1], s.lastRoll)
		if ok && !s.blockedAt(to) {
			movable = append(movable, token)
		}
	}
	return movable
}

// blockedAt reports whether an opponent pair holds the unsafe square the mover would land on.
func (s state) blockedAt(to Progress) bool {
	square, unsafe := s.unsafeSquare(to)
	if !unsafe {
		return false
	}
	return slices.ContainsFunc(s.order, func(seat game.Seat) bool { return len(s.tokensOn(seat, square)) >= 2 })
}

// captureAt is the lone opponent token on the unsafe square the mover would land on.
func (s state) captureAt(to Progress) (game.Seat, Token, bool) {
	square, unsafe := s.unsafeSquare(to)
	if !unsafe {
		return "", 0, false
	}
	for _, seat := range s.order {
		if found := s.tokensOn(seat, square); len(found) == 1 {
			return seat, found[0], true
		}
	}
	return "", 0, false
}

func (s state) unsafeSquare(to Progress) (int, bool) {
	square, onTrack := trackSquare(s.turn, to)
	return square, onTrack && !isSafe(square)
}

// tokensOn lists an opponent's tokens on a track square; the mover's own never count.
func (s state) tokensOn(seat game.Seat, square int) []Token {
	if seat == s.turn {
		return nil
	}
	var found []Token
	for i, p := range s.tokens[seat] {
		if at, ok := trackSquare(seat, p); ok && at == square {
			found = append(found, Token(i+1))
		}
	}
	return found
}

func withProgress(tokens Tokens, token Token, p Progress) Tokens {
	tokens[token-1] = p
	return tokens
}

func allHome(tokens Tokens) bool {
	return !slices.ContainsFunc(tokens[:], func(p Progress) bool { return p != ProgressHome })
}

func (s state) View(game.Seat) View {
	movable := []Token{}
	if s.phase == PhaseMove {
		movable = s.movable()
	}
	starts := make(map[game.Seat]int, len(s.order))
	for _, seat := range s.order {
		starts[seat] = trackStart(seat)
	}
	return View{
		Starts:     starts,
		Progress:   maps.Clone(s.tokens),
		LastRoll:   s.lastRoll,
		SixesInRow: s.sixes,
		Phase:      s.phase,
		Movable:    movable,
	}
}

func (s state) ToAct() []game.Seat {
	if s.finished {
		return nil
	}
	return []game.Seat{s.turn}
}

func (s state) Outcome() (game.Result, bool) { return s.result, s.finished }

// Eliminate removes the seat and its tokens; the last seat standing wins.
func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	if s.finished || !slices.Contains(s.order, seat) {
		return s, nil
	}
	next := s
	if s.turn == seat {
		next = next.passTurn()
	}
	next.order = slices.DeleteFunc(slices.Clone(s.order), func(o game.Seat) bool { return o == seat })
	next.tokens = maps.Clone(s.tokens)
	delete(next.tokens, seat)
	if len(next.order) == 1 {
		next = next.finish(game.ForfeitToRemaining(s.order, seat, reason))
	}
	return next, nil
}
