package sudoku

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

func newState(t *testing.T, opts Opts, seed game.Seed) state {
	t.Helper()
	return gametest.NewSolo[state](t, Rules{}, opts, solo, seed)
}

func setText(cell int, d Digit) string {
	return fmt.Sprintf("%d%d %d", cell/Size+1, cell%Size+1, d)
}

// solvingSets fills every open cell of s with its solution digit, in row-major order.
func solvingSets(s state) []string {
	var texts []string
	for cell, d := range s.puzzle {
		if d == Empty {
			texts = append(texts, setText(cell, s.solution[cell]))
		}
	}
	return texts
}

func digitsOf(g Grid) string {
	var b strings.Builder
	for _, d := range g {
		b.WriteByte(byte('0' + d))
	}
	return b.String()
}

// solvingScript makes a wrong set, clears it, solves, then tries a set after the finish,
// around two illegal sets at the start.
func solvingScript(t *testing.T, d Difficulty, seed game.Seed) gametest.Script[Opts, Action] {
	t.Helper()
	opts := Opts{Difficulty: d}
	s := newState(t, opts, seed)
	given := slices.IndexFunc(s.puzzle[:], func(d Digit) bool { return d != Empty })
	open := slices.Index(s.puzzle[:], Empty)
	steps := []gametest.Step[Action]{
		{Seat: game.SeatSolo, Action: set(setText(given, Empty)), WantErr: true},
		{Seat: game.SeatSolo, Action: set("00 1"), WantErr: true},
		{Seat: game.SeatSolo, Action: set(setText(open, s.solution[open]%Size+1))},
		{Seat: game.SeatSolo, Action: set(setText(open, Empty))},
	}
	for _, text := range solvingSets(s) {
		steps = append(steps, gametest.Step[Action]{Seat: game.SeatSolo, Action: set(text)})
	}
	steps = append(steps, gametest.Step[Action]{Seat: game.SeatSolo, Action: set(setText(open, Empty)), WantErr: true})
	return gametest.Script[Opts, Action]{
		Name:       string(d),
		Opts:       opts,
		Challenger: solo,
		Seed:       seed,
		Steps:      steps,
		WantResult: new(game.WinFor(ReasonSolved, game.SeatSolo)),
		Secrets:    []string{digitsOf(s.solution)},
	}
}

func difficulties() []Difficulty {
	return []Difficulty{DifficultyEasy, DifficultyMedium, DifficultyHard}
}

const conformanceSeed game.Seed = 3

func TestConformance(t *testing.T) {
	var scripts []gametest.Script[Opts, Action]
	for _, d := range difficulties() {
		scripts = append(scripts, solvingScript(t, d, conformanceSeed))
	}
	gametest.Run(t, Rules{}, scripts)
}
