package fifteen

import (
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
)

func newGame(t *testing.T, size Size, seed game.Seed) state {
	t.Helper()
	return gametest.NewSolo[state](t, Rules{}, Opts{Size: size}, solo, seed)
}

// solvingActions solves the seed's shuffle, each slide sent as a move by direction when
// the tile's index in the solution is even, and by number when odd.
func solvingActions(t *testing.T, size Size, seed game.Seed) []Action {
	t.Helper()
	s := newGame(t, size, seed)
	var actions []Action
	for i, tile := range solve(t, s.board) {
		action := slide(tile)
		if i%2 == 0 {
			for _, m := range s.board.movable() {
				if m.Tile == tile {
					action = move(m.Direction)
				}
			}
		}
		actions = append(actions, action)
		s.board = s.board.slid(tile)
	}
	return actions
}

func steps(actions []Action) []gametest.Step[Action] {
	out := make([]gametest.Step[Action], len(actions))
	for i, action := range actions {
		out[i] = gametest.Step[Action]{Seat: game.SeatSolo, Action: action}
	}
	return out
}

func refused(action Action) gametest.Step[Action] {
	return gametest.Step[Action]{Seat: game.SeatSolo, Action: action, WantErr: true}
}

const conformanceSeed game.Seed = 4

func TestConformance(t *testing.T) {
	solved := game.WinFor(ReasonSolved, game.SeatSolo)
	var scripts []gametest.Script[Opts, Action]
	for _, size := range sizes() {
		s := newGame(t, size, conformanceSeed)
		stuck := Action{Kind: ActionSlide, Tile: "99"}
		play := append([]gametest.Step[Action]{refused(stuck), refused(Action{Kind: ActionMove, Direction: "north"})},
			steps(solvingActions(t, size, conformanceSeed))...)
		play = append(play, refused(slide(s.board.movable()[0].Tile)))
		scripts = append(scripts, gametest.Script[Opts, Action]{
			Name: "solves " + string(size), Opts: Opts{Size: size}, Challenger: solo, Seed: conformanceSeed,
			Steps: play, WantResult: &solved,
		})
	}
	scripts = append(scripts, gametest.Script[Opts, Action]{
		Name: "unfinished", Opts: Opts{Size: Size4}, Challenger: solo, Seed: 5,
		Steps: steps(solvingActions(t, Size4, 5)[:3]),
	})
	gametest.Run(t, Rules{}, scripts)
}
