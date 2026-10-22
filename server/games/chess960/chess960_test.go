package chess960

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
)

// Seeds whose first draw is a start position the tests play from.
const (
	seedStandardArray game.Seed = 1325 // 518, RNBQKBNR
	seedRQNBNKBR      game.Seed = 5    // 413
)

func letters(rank chess.BackRank) string {
	const kindLetters = "?PNBRQK"
	var b strings.Builder
	for _, kind := range rank {
		b.WriteByte(kindLetters[kind])
	}
	return b.String()
}

func TestBackRankFollowsScharnaglsNumbering(t *testing.T) {
	tests := []struct {
		n    StartPosition
		want string
	}{
		{0, "BBQNNRKR"}, {1, "BQNBNRKR"}, {2, "BQNNRBKR"}, {100, "QBBNRNKR"},
		{413, "RQNBNKBR"}, {518, "RNBQKBNR"}, {777, "QRKBBNRN"}, {959, "RKRNNQBB"},
	}
	for _, tc := range tests {
		if got := letters(backRank(tc.n)); got != tc.want {
			t.Errorf("position %d: %s, want %s", tc.n, got, tc.want)
		}
	}
	all := make([]string, positionCount)
	for n := range StartPosition(positionCount) {
		all[n] = letters(backRank(n))
	}
	// The digest of every back rank, a line each from 0, as python-chess 1.11.2's
	// Board.from_chess960_pos numbers them.
	const pythonChessDigest = "7bd70cbf530350170cd7ee4e5676ee32dd2374c8a54491c4240b1e5ba873716d"
	if sum := sha256.Sum256([]byte(strings.Join(all, "\n"))); hex.EncodeToString(sum[:]) != pythonChessDigest {
		t.Errorf("the numbering differs from python-chess's")
	}
}

func TestEveryStartPositionIsAChess960One(t *testing.T) {
	seen := map[string]bool{}
	for n := range StartPosition(positionCount) {
		rank := backRank(n)
		text := letters(rank)
		bishops := slices.Collect(func(yield func(int) bool) {
			for file, kind := range rank {
				if kind == chess.Bishop && !yield(file) {
					return
				}
			}
		})
		if len(bishops) != 2 || bishops[0]%2 == bishops[1]%2 {
			t.Errorf("position %d, %s: bishops not on opposite colours", n, text)
		}
		if _, err := chess.Chess960Position(rank); err != nil {
			t.Errorf("position %d: %v", n, err)
		}
		seen[text] = true
	}
	if len(seen) != positionCount {
		t.Errorf("%d distinct back ranks, want %d", len(seen), positionCount)
	}
}

func newGame(t *testing.T, seed game.Seed, color chess.ColorChoice) (game.ErasedState, map[game.UserID]game.Seat) {
	t.Helper()
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(json.RawMessage(`{"color":"` + string(color) + `"}`))
	if err != nil {
		t.Fatal(err)
	}
	st, seats, err := g.New(opts, alice, []game.UserID{bob}, game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	return st, seats
}

func play(t *testing.T, st game.ErasedState, line string) game.ErasedState {
	t.Helper()
	for i, move := range strings.Fields(line) {
		raw, err := json.Marshal(chess.Action{Kind: chess.ActionMove, Move: move})
		if err != nil {
			t.Fatal(err)
		}
		if st, _, err = st.Apply(st.ToAct()[0], raw, game.StepRNG(1, game.Seq(i+1))); err != nil {
			t.Fatalf("%s: %v", move, err)
		}
	}
	return st
}

func viewAs(t *testing.T, st game.ErasedState, seat game.Seat) View {
	t.Helper()
	raw, err := st.View(seat)
	if err != nil {
		t.Fatal(err)
	}
	var v View
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestTheStartPositionComesFromTheSeed(t *testing.T) {
	st, _ := newGame(t, seedRQNBNKBR, chess.ColorWhite)
	v := viewAs(t, st, chess.SeatWhite)
	const want = "rqnbnkbr/pppppppp/8/8/8/8/PPPPPPPP/RQNBNKBR w KQkq - 0 1"
	if v.StartPosition != 413 || v.FEN != want {
		t.Errorf("start %d, fen %q; want 413 %q", v.StartPosition, v.FEN, want)
	}
	if len(v.LegalMoves) != 20 {
		t.Errorf("%d legal moves at the start, want 20", len(v.LegalMoves))
	}
	again, _ := newGame(t, seedRQNBNKBR, chess.ColorWhite)
	other, _ := newGame(t, seedStandardArray, chess.ColorWhite)
	if viewAs(t, again, chess.SeatWhite).StartPosition != 413 || viewAs(t, other, chess.SeatWhite).StartPosition != 518 {
		t.Errorf("the start position does not follow the seed")
	}
}

func TestCastlingInAGame(t *testing.T) {
	st, _ := newGame(t, seedRQNBNKBR, chess.ColorWhite)
	st = play(t, st, "f4 f5 Bf2 Bf7")
	if v := viewAs(t, st, chess.SeatWhite); !slices.Contains(v.LegalMoves, "f1h1") || !slices.Contains(v.LegalMoves, "f1g1") {
		t.Errorf("legal moves %v: want both the castle f1h1 and the king's step f1g1", v.LegalMoves)
	}
	st = play(t, st, "O-O f8h8")
	v := viewAs(t, st, chess.SeatWhite)
	const want = "rqnbnrk1/pppppbpp/8/5p2/5P2/8/PPPPPBPP/RQNBNRK1 w - - 4 4"
	if v.FEN != want || !slices.Equal(v.HistorySAN[4:], []string{"O-O", "O-O"}) {
		t.Errorf("fen %s, history %v; want %s after O-O O-O", v.FEN, v.HistorySAN, want)
	}
}

func TestInfoIsChesssUnderItsOwnType(t *testing.T) {
	info, chessInfo := Rules{}.Info(), chess.Rules{}.Info()
	if info.Type != TypeChess960 || info.Type == chessInfo.Type {
		t.Errorf("type %q", info.Type)
	}
	i := slices.IndexFunc(info.Actions, func(a game.ActionSpec) bool { return a.Kind == chess.ActionMove })
	if i < 0 || info.Actions[i].Input.Pattern != chess.MovePattern || !strings.Contains(info.Actions[i].Input.Description, "f1h1") {
		t.Fatalf("move action %+v", info.Actions)
	}
	if chessInfo.Actions[i].Input.Description == info.Actions[i].Input.Description {
		t.Errorf("chess's description changed with chess960's")
	}
	if !cmp.Equal(info.Options, chessInfo.Options) || !slices.Equal(info.Capabilities, chessInfo.Capabilities) {
		t.Errorf("options or capabilities differ from chess's")
	}
}

func TestColorOption(t *testing.T) {
	if _, seats := newGame(t, 1, chess.ColorWhite); seats[alice] != chess.SeatWhite {
		t.Errorf("white: %v", seats)
	}
	if _, seats := newGame(t, 1, chess.ColorBlack); seats[alice] != chess.SeatBlack {
		t.Errorf("black: %v", seats)
	}
	seatsBySeed := map[game.Seat]bool{}
	for seed := range game.Seed(8) {
		_, first := newGame(t, seed, chess.ColorRandom)
		_, second := newGame(t, seed, chess.ColorRandom)
		if first[alice] != second[alice] {
			t.Errorf("seed %d: random not deterministic: %v vs %v", seed, first, second)
		}
		seatsBySeed[first[alice]] = true
	}
	if len(seatsBySeed) != 2 {
		t.Errorf("random gave alice only %v over 8 seeds", seatsBySeed)
	}
}

func TestPGNTagsTheVariantAndTheStart(t *testing.T) {
	st, _ := newGame(t, seedStandardArray, chess.ColorWhite)
	st = play(t, st, "f3 e5 g4 Qh4#")
	result, finished := st.Outcome()
	if !finished {
		t.Fatal("fool's mate did not finish")
	}
	ctx := game.ExportContext{
		Players:   []game.Player{{User: alice, Seat: chess.SeatWhite}, {User: bob, Seat: chess.SeatBlack}},
		StartedAt: time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC),
		Result:    result,
	}
	want := strings.Join([]string{
		`[Event "games.nvim"]`,
		`[Site "?"]`,
		`[Date "2026.10.21"]`,
		`[Round "-"]`,
		`[White "alice"]`,
		`[Black "bob"]`,
		`[Result "0-1"]`,
		`[Variant "Chess960"]`,
		`[SetUp "1"]`,
		`[FEN "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"]`,
		"",
		"1. f3 e5 2. g4 Qh4# 0-1",
		"",
	}, "\n")
	if got := string(st.Exports(ctx)[chess.PGNFile]); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestConcludeStopsOfferingMoves(t *testing.T) {
	st, _ := newGame(t, seedRQNBNKBR, chess.ColorWhite)
	st = st.Conclude(game.Result{Kind: game.ResultDraw, Reason: game.ReasonDrawAgreed})
	if v := viewAs(t, st, chess.SeatWhite); len(v.LegalMoves) != 0 || len(st.ToAct()) != 0 || v.StartPosition != 413 {
		t.Errorf("after a draw: legal %v, to act %v, start %d", v.LegalMoves, st.ToAct(), v.StartPosition)
	}
}

func moves(sans string) []gametest.Step[chess.Action] {
	var steps []gametest.Step[chess.Action]
	seat := chess.SeatWhite
	for san := range strings.FieldsSeq(sans) {
		steps = append(steps, gametest.Step[chess.Action]{Seat: seat, Action: chess.Action{Kind: chess.ActionMove, Move: san}})
		seat = map[game.Seat]game.Seat{chess.SeatWhite: chess.SeatBlack, chess.SeatBlack: chess.SeatWhite}[seat]
	}
	return steps
}

func TestConformance(t *testing.T) {
	script := func(name string, seed game.Seed, steps []gametest.Step[chess.Action], want *game.Result) gametest.Script[chess.Opts, chess.Action] {
		return gametest.Script[chess.Opts, chess.Action]{
			Name: name, Opts: chess.Opts{TimeControl: game.TimeControlNone, Color: chess.ColorWhite},
			Seed: seed, Steps: steps, WantResult: want,
		}
	}
	mated := game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{chess.SeatBlack}, Reason: chess.ReasonCheckmate}
	notAKingMove := gametest.Step[chess.Action]{Seat: chess.SeatBlack, Action: chess.Action{Kind: chess.ActionMove, Move: "e8g8"}, WantErr: true}
	castled := slices.Insert(moves("f4 f5 Bf2 Bf7 f1h1 O-O"), 5, notAKingMove)
	gametest.Run(t, Rules{}, []gametest.Script[chess.Opts, chess.Action]{
		script("fools mate from the standard array", seedStandardArray, moves("f3 e5 g4 Qh4#"), &mated),
		script("both sides castle", seedRQNBNKBR, castled, nil),
	})
}
