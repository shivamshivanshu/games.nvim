package mastermind

import (
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

type state struct {
	game.Ending
	secret  Code
	guesses []Guess
}

func (s state) Apply(seat game.Seat, action Action, _ *rand.Rand) (game.State[Action, View], []game.Event, error) {
	if action.Kind != ActionGuess {
		return nil, nil, game.UnknownAction(action.Kind)
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
		next.Ending = next.Concluded(game.WinFor(ReasonSolved, game.SeatSolo))
	case len(next.guesses) == MaxGuesses:
		next.Ending = next.Concluded(game.LossBy(ReasonOutOfGuesses))
	}
	return next
}

func (s state) View(game.Seat) View {
	view := View{
		Palette:    strings.Split(Palette, ""),
		CodeLength: CodeLength,
		Guesses:    xslices.Clone(s.guesses),
		Remaining:  MaxGuesses - len(s.guesses),
	}
	if s.Over() {
		view.Secret = s.secret
	}
	return view
}

func (s state) ToAct() []game.Seat { return s.ToActOf(game.SeatSolo) }

func (s state) Eliminate(seat game.Seat, reason game.ResultReason) (game.State[Action, View], []game.Event) {
	return s.Conclude(game.ForfeitToRemaining([]game.Seat{game.SeatSolo}, seat, reason)), nil
}

// Conclude keeps a result already reached; otherwise result ends the game and reveals the secret.
func (s state) Conclude(result game.Result) game.State[Action, View] {
	s.Ending = s.Concluded(result)
	return s
}
