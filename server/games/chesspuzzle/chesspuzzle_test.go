package chesspuzzle

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
)

const solo game.UserID = "alice"

// scholarsFEN is black to move after 1. e4 e5 2. Bc4 Nc6 3. Qh5, as a Lichess
// row gives it: black's Nf6 sets up white's Qxf7#.
const scholarsFEN = "r1bqkbnr/pppp1ppp/2n5/4p2Q/2B1P3/8/PPPP1PPP/RNB1K1NR b KQkq - 3 3"

var scholars = Opts{Mode: ModePuzzle, FEN: scholarsFEN, Moves: "g8f6 h5f7", Player: SideWhite}

// backRankFEN mates on a8 with the rook, or on d8 with the queen.
const backRankFEN = "7k/6pp/8/8/8/8/6PP/R2Q3K w - - 0 1"

var backRank = Opts{Mode: ModePuzzle, FEN: backRankFEN, Moves: "Ra8#", Player: SideWhite}

var openingStudy = Opts{Mode: ModeStudy, Moves: "e4 e5 Nf3 Nc6 Bb5", Player: SideWhite}

func move(text string) Action { return Action{Kind: ActionMove, Text: text} }

func reveal() Action { return Action{Kind: ActionReveal} }

func start(t *testing.T, opts Opts) state {
	t.Helper()
	return open(t, opts).state
}

func open(t *testing.T, opts Opts) opened {
	t.Helper()
	st, seats, err := Rules{}.New(opts, solo, nil, nil)
	if err != nil {
		t.Fatalf("New(%+v): %v", opts, err)
	}
	if diff := cmp.Diff(map[game.UserID]game.Seat{solo: game.SeatSolo}, seats); diff != "" {
		t.Fatalf("seats (-want +got):\n%s", diff)
	}
	return st.(opened)
}

func step(action Action) gametest.Step[Action] {
	return gametest.Step[Action]{Seat: game.SeatSolo, Action: action}
}

func failingStep(action Action) gametest.Step[Action] {
	return gametest.Step[Action]{Seat: game.SeatSolo, Action: action, WantErr: true}
}

func TestConformance(t *testing.T) {
	gametest.Run(t, Rules{}, []gametest.Script[Opts, Action]{
		{
			Name:       "puzzle solved",
			Opts:       scholars,
			Challenger: solo,
			Steps:      []gametest.Step[Action]{failingStep(move("Kd8")), failingStep(reveal()), step(move("Qxf7#"))},
			WantResult: &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{game.SeatSolo}, Reason: ReasonSolved},
		},
		{
			Name:       "puzzle wrong move",
			Opts:       scholars,
			Challenger: solo,
			Steps:      []gametest.Step[Action]{step(move("Qxe5+")), failingStep(move("Qxf7#"))},
			WantResult: &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{}, Reason: ReasonWrongMove},
		},
		{
			Name:       "puzzle alternative mate",
			Opts:       backRank,
			Challenger: solo,
			Steps:      []gametest.Step[Action]{step(move("Qd8#"))},
			WantResult: &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{game.SeatSolo}, Reason: ReasonSolved},
		},
		{
			Name:       "study completed",
			Opts:       openingStudy,
			Challenger: solo,
			Steps:      []gametest.Step[Action]{step(move("e4")), step(move("d4")), step(reveal()), step(move("Bb5"))},
			WantResult: &game.Result{Kind: game.ResultDecisive, Winners: []game.Seat{game.SeatSolo}, Reason: ReasonCompleted},
		},
	})
}

func TestNewRejectsBadLinesNamingThePly(t *testing.T) {
	shuffle := strings.Repeat("Nf3 Nf6 Ng1 Ng8 ", MaxPlies/4)
	tests := []struct {
		name string
		opts Opts
		want string
	}{
		{"fen does not parse", Opts{Mode: ModePuzzle, FEN: "8/8 w", Moves: "e4", Player: SideWhite}, "fen: want 6 fields, got 2"},
		{"empty line", Opts{Mode: ModePuzzle, Player: SideWhite}, "the line holds 0 plies, want 1 to 600"},
		{"too long a line", Opts{Mode: ModeStudy, Moves: shuffle + "Nf3", Player: SideWhite}, "the line holds 601 plies, want 1 to 600"},
		{"illegal ply", Opts{Mode: ModeStudy, Moves: "e4 e5 Nf6", Player: SideWhite}, "ply 3: illegal move: Nf6"},
		{"illegal first ply from a fen", Opts{Mode: ModePuzzle, FEN: scholarsFEN, Moves: "h5f7", Player: SideWhite}, "ply 1: illegal move: h5f7"},
		{"ply after mate", Opts{Mode: ModePuzzle, FEN: backRankFEN, Moves: "Ra8 h6", Player: SideWhite}, "ply 2: illegal move: h6"},
		{"no move for the player", Opts{Mode: ModePuzzle, Moves: "e4", Player: SideBlack}, "the line has no move for black"},
		{"unknown mode", Opts{Mode: "blitz", Moves: "e4", Player: SideWhite}, `unknown mode "blitz"`},
		{"unknown player", Opts{Mode: ModeStudy, Moves: "e4", Player: "both"}, `unknown player "both"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Rules{}.New(tt.opts, solo, nil, nil)
			if !errors.As(err, new(game.BadRequestError)) || err.Error() != tt.want {
				t.Errorf("New: got %v, want bad request %q", err, tt.want)
			}
		})
	}
}

func TestOptionsTakeTheLongestLineThroughTheCatalogue(t *testing.T) {
	longest := strings.TrimSpace(strings.Repeat("Qh4xe1+ ", MaxPlies))
	if len(longest) != movesMaxLen {
		t.Fatalf("movesMaxLen = %d, want %d: room for %d plies of the widest SAN", movesMaxLen, len(longest), MaxPlies)
	}
	g := game.Adapt(Rules{})
	shuffle := strings.TrimSpace(strings.Repeat("Nf3 Nf6 Ng1 Ng8 ", MaxPlies/4))
	raw, err := json.Marshal(map[game.OptionName]string{OptionMode: string(ModeStudy), OptionMoves: shuffle})
	if err != nil {
		t.Fatal(err)
	}
	opts, err := g.ResolveOptions(raw)
	if err != nil {
		t.Fatalf("ResolveOptions: %v", err)
	}
	if _, _, err := g.New(opts, solo, nil, nil); err != nil {
		t.Errorf("New with %d plies: %v", MaxPlies, err)
	}
}

func TestStartPlaysTheOtherSideUpToTheSolversFirstMove(t *testing.T) {
	tests := []struct {
		name        string
		opts        Opts
		want        View
		wantReplies []game.Event
	}{
		{
			name:        "a lichess row's setup move",
			opts:        scholars,
			wantReplies: []game.Event{RepliedEvent{By: game.SeatSolo, SAN: "Nf6"}},
			want: View{
				FEN:        "r1bqkb1r/pppp1ppp/2n2n2/4p2Q/2B1P3/8/PPPP1PPP/RNB1K1NR w KQkq - 4 4",
				HistorySAN: []string{"Nf6"},
				LastMove:   &chess.MoveSquares{From: "g8", To: "f6"},
				FirstPly:   5,
				Mode:       ModePuzzle,
				Remaining:  1,
			},
		},
		{
			name:        "black studies from the start",
			opts:        Opts{Mode: ModeStudy, Moves: "e4 e5 Nf3", Player: SideBlack},
			wantReplies: []game.Event{RepliedEvent{By: game.SeatSolo, SAN: "e4"}},
			want: View{
				FEN:        "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
				HistorySAN: []string{"e4"},
				LastMove:   &chess.MoveSquares{From: "e2", To: "e4"},
				Mode:       ModeStudy,
				Remaining:  1,
			},
		},
		{
			name: "white studies from the start",
			opts: openingStudy,
			want: View{FEN: chess.StartFEN, HistorySAN: []string{}, Mode: ModeStudy, Remaining: 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opening := open(t, tt.opts)
			if diff := cmp.Diff(tt.wantReplies, opening.OpeningEvents()); diff != "" {
				t.Errorf("opening events (-want +got):\n%s", diff)
			}
			view := opening.View(game.SeatSolo)
			if len(view.LegalMoves) == 0 {
				t.Errorf("the solver to move has no legal moves in the view")
			}
			view.LegalMoves = nil
			tt.want.Solution = []string{}
			if diff := cmp.Diff(tt.want, view); diff != "" {
				t.Errorf("view (-want +got):\n%s", diff)
			}
		})
	}
}

func TestACorrectMoveIsAnsweredByTheLine(t *testing.T) {
	s, events := gametest.MustApply(t, Rules{}.Info(), start(t, openingStudy), game.SeatSolo, move("e2e4"), nil)
	want := []game.Event{MovedEvent{By: game.SeatSolo, SAN: "e4"}, RepliedEvent{By: game.SeatSolo, SAN: "e5"}}
	if diff := cmp.Diff(want, events); diff != "" {
		t.Errorf("events (-want +got):\n%s", diff)
	}
	view := s.View(game.SeatSolo)
	if diff := cmp.Diff([]string{"e4", "e5"}, view.HistorySAN); diff != "" {
		t.Errorf("history (-want +got):\n%s", diff)
	}
	if view.Remaining != 2 || len(s.ToAct()) != 1 {
		t.Errorf("remaining = %d, to act %v; want 2 and the solver", view.Remaining, s.ToAct())
	}
}

func TestPuzzleEndings(t *testing.T) {
	tests := []struct {
		name       string
		opts       Opts
		input      string
		wantEvents []game.Event
		wantResult game.Result
		wantFEN    string
		wantMisses int
		wantLeft   int
	}{
		{
			name:       "the line's mate solves it",
			opts:       scholars,
			input:      "qxf7",
			wantEvents: []game.Event{MovedEvent{By: game.SeatSolo, SAN: "Qxf7#"}},
			wantResult: game.WinFor(ReasonSolved, game.SeatSolo),
			wantFEN:    "r1bqkb1r/pppp1Qpp/2n2n2/4p3/2B1P3/8/PPPP1PPP/RNB1K1NR b KQkq - 0 4",
		},
		{
			name:       "an alternative mate solves it",
			opts:       backRank,
			input:      "Qd8",
			wantEvents: []game.Event{MovedEvent{By: game.SeatSolo, SAN: "Qd8#"}},
			wantResult: game.WinFor(ReasonSolved, game.SeatSolo),
			wantFEN:    "3Q3k/6pp/8/8/8/8/6PP/R6K b - - 1 1",
		},
		{
			name:       "a wrong move fails it and leaves the board",
			opts:       scholars,
			input:      "Qxe5+",
			wantEvents: []game.Event{MissedEvent{By: game.SeatSolo, SAN: "Qxe5+"}},
			wantResult: game.LossBy(ReasonWrongMove),
			wantFEN:    "r1bqkb1r/pppp1ppp/2n2n2/4p2Q/2B1P3/8/PPPP1PPP/RNB1K1NR w KQkq - 4 4",
			wantMisses: 1,
			wantLeft:   1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, events := gametest.MustApply(t, Rules{}.Info(), start(t, tt.opts), game.SeatSolo, move(tt.input), nil)
			if diff := cmp.Diff(tt.wantEvents, events); diff != "" {
				t.Errorf("events (-want +got):\n%s", diff)
			}
			result, finished := s.Outcome()
			if diff := cmp.Diff(tt.wantResult, result); !finished || diff != "" {
				t.Errorf("finished = %t, result (-want +got):\n%s", finished, diff)
			}
			view := s.View(game.SeatSolo)
			if view.FEN != tt.wantFEN || view.Misses != tt.wantMisses || view.Remaining != tt.wantLeft {
				t.Errorf("fen %q, misses %d, remaining %d; want %q, %d, %d", view.FEN, view.Misses, view.Remaining, tt.wantFEN, tt.wantMisses, tt.wantLeft)
			}
		})
	}
}

func TestStudyMissesWaitForTheRightMove(t *testing.T) {
	s, _ := gametest.MustApply(t, Rules{}.Info(), start(t, openingStudy), game.SeatSolo, move("e4"), nil)
	before := s.View(game.SeatSolo)

	s, events := gametest.MustApply(t, Rules{}.Info(), s, game.SeatSolo, move("Nc3"), nil)
	if diff := cmp.Diff([]game.Event{MissedEvent{By: game.SeatSolo, SAN: "Nc3"}}, events); diff != "" {
		t.Errorf("miss events (-want +got):\n%s", diff)
	}
	missed := s.View(game.SeatSolo)
	if missed.Misses != 1 || missed.FEN != before.FEN || !cmp.Equal(missed.HistorySAN, before.HistorySAN) {
		t.Errorf("after a miss: misses %d, fen %q, history %v; want 1 and the position unchanged", missed.Misses, missed.FEN, missed.HistorySAN)
	}

	s, events = gametest.MustApply(t, Rules{}.Info(), s, game.SeatSolo, move("Qh5"), nil)
	if diff := cmp.Diff([]game.Event{MissedEvent{By: game.SeatSolo, SAN: "Qh5"}}, events); diff != "" {
		t.Errorf("a miss that is not mate in study (-want +got):\n%s", diff)
	}

	s, events = gametest.MustApply(t, Rules{}.Info(), s, game.SeatSolo, reveal(), nil)
	want := []game.Event{RevealedEvent{By: game.SeatSolo, SAN: "Nf3"}, RepliedEvent{By: game.SeatSolo, SAN: "Nc6"}}
	if diff := cmp.Diff(want, events); diff != "" {
		t.Errorf("reveal events (-want +got):\n%s", diff)
	}

	s, events = gametest.MustApply(t, Rules{}.Info(), s, game.SeatSolo, move("Bb5"), nil)
	if diff := cmp.Diff([]game.Event{MovedEvent{By: game.SeatSolo, SAN: "Bb5"}}, events); diff != "" {
		t.Errorf("last move events (-want +got):\n%s", diff)
	}
	result, finished := s.Outcome()
	if diff := cmp.Diff(game.WinFor(ReasonCompleted, game.SeatSolo), result); !finished || diff != "" {
		t.Errorf("finished = %t, result (-want +got):\n%s", finished, diff)
	}
	final := s.View(game.SeatSolo)
	if final.Misses != 3 || final.Remaining != 0 {
		t.Errorf("final view: misses %d, remaining %d; want 3 and 0", final.Misses, final.Remaining)
	}
}

func TestStudyDoesNotTakeAnAlternativeMate(t *testing.T) {
	opts := backRank
	opts.Mode = ModeStudy
	s, events := gametest.MustApply(t, Rules{}.Info(), start(t, opts), game.SeatSolo, move("Qd8"), nil)
	if diff := cmp.Diff([]game.Event{MissedEvent{By: game.SeatSolo, SAN: "Qd8#"}}, events); diff != "" {
		t.Errorf("events (-want +got):\n%s", diff)
	}
	if _, finished := s.Outcome(); finished || s.View(game.SeatSolo).FEN != backRankFEN {
		t.Errorf("an alternative mate in study must be a miss that keeps the position")
	}
}

func TestIllegalInputIsRejectedWithoutAMiss(t *testing.T) {
	tests := []struct {
		name   string
		opts   Opts
		action Action
		want   string
	}{
		{"an illegal move", openingStudy, move("e5"), "illegal move: e5"},
		{"not a move", openingStudy, move("hello"), "illegal move: hello"},
		{"reveal in puzzle mode", scholars, reveal(), "reveal is for study mode only"},
		{"an unknown kind", openingStudy, Action{Kind: "hint"}, "unknown action: hint"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := start(t, tt.opts).Apply(game.SeatSolo, tt.action, nil)
			if !errors.As(err, new(game.IllegalActionError)) || err.Error() != tt.want {
				t.Errorf("Apply: got %v, want illegal action %q", err, tt.want)
			}
		})
	}
}

func TestSolutionAndLegalMovesFollowTheEnd(t *testing.T) {
	ongoing := start(t, scholars)
	solved, _ := gametest.MustApply(t, Rules{}.Info(), ongoing, game.SeatSolo, move("Qxf7#"), nil)
	gaveUp := ongoing.Conclude(game.ForfeitToRemaining([]game.Seat{game.SeatSolo}, game.SeatSolo, game.ReasonResign))
	tests := []struct {
		name          string
		state         game.State[Action, View]
		wantSolution  []string
		wantLegalMove bool
	}{
		{"ongoing", ongoing, []string{}, true},
		{"solved", solved, []string{"Nf6", "Qxf7#"}, false},
		{"given up", gaveUp, []string{"Nf6", "Qxf7#"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := tt.state.View(game.SeatSolo)
			if diff := cmp.Diff(tt.wantSolution, view.Solution); diff != "" {
				t.Errorf("solution (-want +got):\n%s", diff)
			}
			if got := len(view.LegalMoves) > 0; got != tt.wantLegalMove || view.LegalMoves == nil {
				t.Errorf("legal moves %v; want some: %t, never null", view.LegalMoves, tt.wantLegalMove)
			}
		})
	}
	if got := ongoing.View(game.SeatSolo).LegalMoves; !strings.Contains(strings.Join(got, " "), "h5f7") {
		t.Errorf("legal moves %v lack the UCI h5f7", got)
	}
}

// mirroredBackRank has black, to move second, mate on a1 with the rook or on
// d1 with the queen once white has played two quiet pawn moves.
var mirroredBackRank = Opts{Mode: ModePuzzle, FEN: "r2q3k/6pp/8/8/8/8/4P1PP/7K w - - 0 1", Moves: "e3 h6 e4 Ra1#", Player: SideBlack}

func TestAPuzzleAnswersACorrectMoveBeforeTheLast(t *testing.T) {
	s, events := gametest.MustApply(t, Rules{}.Info(), start(t, mirroredBackRank), game.SeatSolo, move("h6"), nil)
	want := []game.Event{MovedEvent{By: game.SeatSolo, SAN: "h6"}, RepliedEvent{By: game.SeatSolo, SAN: "e4"}}
	if diff := cmp.Diff(want, events); diff != "" {
		t.Errorf("events (-want +got):\n%s", diff)
	}
	if _, finished := s.Outcome(); finished || len(s.ToAct()) != 1 || s.View(game.SeatSolo).Remaining != 1 {
		t.Errorf("finished = %t, to act %v, remaining %d; want the solver to find the last move", finished, s.ToAct(), s.View(game.SeatSolo).Remaining)
	}
}

func TestStatesBranchedFromOneDoNotShareHistory(t *testing.T) {
	s, _ := gametest.MustApply(t, Rules{}.Info(), start(t, mirroredBackRank), game.SeatSolo, move("h6"), nil)
	queenMate, _ := gametest.MustApply(t, Rules{}.Info(), s, game.SeatSolo, move("Qd1"), nil)
	rookMate, _ := gametest.MustApply(t, Rules{}.Info(), s, game.SeatSolo, move("Ra1"), nil)
	for _, tt := range []struct {
		state state
		want  string
	}{{queenMate, "Qd1#"}, {rookMate, "Ra1#"}} {
		history := tt.state.View(game.SeatSolo).HistorySAN
		if last := history[len(history)-1]; last != tt.want {
			t.Errorf("history %v ends in %s, want %s", history, last, tt.want)
		}
	}
}

func TestEndingsScoreTheSolverByTheCoresWinOrLoss(t *testing.T) {
	ongoing := start(t, scholars)
	solved, _ := gametest.MustApply(t, Rules{}.Info(), ongoing, game.SeatSolo, move("Qxf7#"), nil)
	failed, _ := gametest.MustApply(t, Rules{}.Info(), ongoing, game.SeatSolo, move("Qxe5+"), nil)
	study, _ := gametest.MustApply(t, Rules{}.Info(), start(t, Opts{Mode: ModeStudy, Moves: "e4", Player: SideWhite}), game.SeatSolo, move("e4"), nil)
	gaveUp, _ := ongoing.Eliminate(game.SeatSolo, game.ReasonResign)
	tests := []struct {
		name  string
		state game.State[Action, View]
		won   bool
	}{
		{"solved", solved, true},
		{"completed", study, true},
		{"wrong move", failed, false},
		{"given up", gaveUp, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, finished := tt.state.Outcome()
			if !finished || result.Kind != game.ResultDecisive || slices.Contains(result.Winners, game.SeatSolo) != tt.won {
				t.Errorf("finished = %t, result %+v; want decisive, solver won: %t", finished, result, tt.won)
			}
			if _, ok := tt.state.(game.Scorer); ok {
				t.Errorf("a Scorer overrides the core's +1 for a win and -1 for a loss")
			}
		})
	}
}
