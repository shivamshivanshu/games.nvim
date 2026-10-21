package mastermind

import (
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

type state struct {
	secret   Code
	guesses  []Guess
	result   game.Result
	finished bool
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if action.Kind != ActionGuess {
		return nil, nil, game.IllegalActionError{Msg: "unknown action: " + string(action.Kind)}
	}
	code, err := parseCode(action.Code)
	if err != nil {
		return nil, nil, err
	}
	latest := grade(code, s.secret)
	return s.with(latest), []game.Event{GuessedEvent{By: seat, Code: latest.Code, Black: latest.Black, White: latest.White}}, nil
}

func (s state) with(latest Guess) state {
	next := state{secret: s.secret, guesses: append(slices.Clip(s.guesses), latest)}
	switch {
	case latest.Black == CodeLength:
		next.result = game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{SeatSolver}, Reason: ReasonSolved}
		next.finished = true
	case len(next.guesses) == MaxGuesses:
		next.result, next.finished = game.ForfeitToRemaining([]game.Seat{SeatSolver}, SeatSolver, ReasonOutOfGuesses), true
	}
	return next
}

func (s state) View(game.Seat) View {
	view := View{
		Palette:    strings.Split(Palette, ""),
		CodeLength: CodeLength,
		Guesses:    append([]Guess{}, s.guesses...),
		Remaining:  MaxGuesses - len(s.guesses),
	}
	if s.finished {
		view.Secret = s.secret
	}
	return view
}

func (s state) ToAct() []game.Seat {
	if s.finished {
		return nil
	}
	return []game.Seat{SeatSolver}
}

func (s state) Outcome() (game.Result, bool) { return s.result, s.finished }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining([]game.Seat{SeatSolver}, seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game and reveals the secret.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	if s.finished {
		return s
	}
	s.result, s.finished = result, true
	return s
}
