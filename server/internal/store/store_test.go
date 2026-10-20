package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"games.nvim/server/game"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
)

var epoch = time.UnixMilli(1_700_000_000_000)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "games.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for _, u := range []game.UserID{alice, bob} {
		if err := s.CreateUser(context.Background(), u, []byte("hash")); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func newGame(id game.GameID) GameRecord {
	return GameRecord{
		ID: id, Type: "chess", Options: json.RawMessage(`{"clock":300}`), Seed: 1<<63 + 7,
		Players:   []game.Player{{User: alice, Seat: "white"}, {User: bob, Seat: "black"}},
		CreatedAt: epoch,
	}
}

func step(seq game.Seq, user game.UserID) StepRecord {
	return StepRecord{
		Seq: seq, User: user, Kind: StepAction, Payload: json.RawMessage(`"e4"`),
		At:               epoch.Add(time.Duration(seq) * time.Second),
		ClockRemainingMS: map[game.UserID]int64{alice: 1000 * int64(seq), bob: 2000},
	}
}

func finish(t *testing.T, s *Store, id game.GameID, seq game.Seq, reason game.ResultReason, outcomes map[game.UserID]Outcome) {
	t.Helper()
	if err := s.FinishGame(context.Background(), id, step(seq, alice), reason, outcomes); err != nil {
		t.Fatal(err)
	}
}

func TestStepsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	rec := newGame("g1")
	if err := s.CreateGame(ctx, rec); err != nil {
		t.Fatal(err)
	}
	for seq := game.Seq(1); seq <= 3; seq++ {
		if err := s.AppendStep(ctx, "g1", step(seq, alice)); err != nil {
			t.Fatal(err)
		}
		rec.Steps = append(rec.Steps, step(seq, alice))
	}

	control := step(4, bob)
	control.Kind = StepControl
	control.Payload = json.RawMessage(`{"op":"resign"}`)
	timeout := StepRecord{Seq: 5, Kind: StepTimeout, At: epoch.Add(time.Minute)}
	for _, st := range []StepRecord{control, timeout} {
		if err := s.AppendStep(ctx, "g1", st); err != nil {
			t.Fatal(err)
		}
		rec.Steps = append(rec.Steps, st)
	}

	got, err := s.LoadOngoing(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]GameRecord{rec}, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestReopenKeepsOngoingGames(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "games.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []game.UserID{alice, bob} {
		if err := s.CreateUser(ctx, u, []byte("hash")); err != nil {
			t.Fatal(err)
		}
	}
	rec := newGame("g1")
	rec.Steps = []StepRecord{step(1, alice), step(2, bob)}
	if err := s.CreateGame(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.LoadOngoing(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]GameRecord{rec}, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestCreateGameRejectsEmptyOptions(t *testing.T) {
	s := openTemp(t)
	rec := newGame("g1")
	rec.Options = nil
	if err := s.CreateGame(context.Background(), rec); !errors.Is(err, ErrMissingOptions) {
		t.Fatalf("err = %v", err)
	}
}

func TestFinishGameRejectsOutcomeForNonPlayer(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	if err := s.CreateGame(ctx, newGame("g1")); err != nil {
		t.Fatal(err)
	}
	err := s.FinishGame(ctx, "g1", step(1, alice), game.ReasonResign, map[game.UserID]Outcome{"mallory": OutcomeWin})
	if err == nil {
		t.Fatal("want error")
	}
	if ongoing, _ := s.LoadOngoing(ctx); len(ongoing) != 1 || len(ongoing[0].Steps) != 0 {
		t.Fatal("finish was not rolled back")
	}
}

func TestFinishGameIsAtomicAndLeavesOngoing(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	if err := s.CreateGame(ctx, newGame("g1")); err != nil {
		t.Fatal(err)
	}
	for seq := game.Seq(1); seq <= 3; seq++ {
		if err := s.AppendStep(ctx, "g1", step(seq, alice)); err != nil {
			t.Fatal(err)
		}
	}
	finish(t, s, "g1", 4, game.ReasonResign, map[game.UserID]Outcome{alice: OutcomeWin, bob: OutcomeLoss})

	ongoing, err := s.LoadOngoing(ctx)
	if err != nil || len(ongoing) != 0 {
		t.Fatalf("ongoing = %v, err = %v", ongoing, err)
	}
	rec, err := s.LoadGame(ctx, "g1")
	if err != nil || len(rec.Steps) != 4 {
		t.Fatalf("steps = %d, err = %v", len(rec.Steps), err)
	}
}

func TestFinishGameRollsBackStepWhenGameMissing(t *testing.T) {
	s := openTemp(t)
	err := s.FinishGame(context.Background(), "nope", step(1, alice), game.ReasonResign, nil)
	if err == nil {
		t.Fatal("want error for unknown game")
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM actions`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("actions rows = %d, err = %v", n, err)
	}
}

func TestStatsExcludeAborted(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	for _, id := range []game.GameID{"win", "draw", "abort"} {
		if err := s.CreateGame(ctx, newGame(id)); err != nil {
			t.Fatal(err)
		}
	}
	finish(t, s, "win", 1, game.ReasonResign, map[game.UserID]Outcome{alice: OutcomeWin, bob: OutcomeLoss})
	finish(t, s, "draw", 1, game.ReasonDrawAgreed, map[game.UserID]Outcome{alice: OutcomeDraw, bob: OutcomeDraw})
	finish(t, s, "abort", 1, game.ReasonAborted, nil)

	stats, h2h, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantStats := []StatRecord{
		{User: alice, Game: "chess", Wins: 1, Draws: 1},
		{User: bob, Game: "chess", Losses: 1, Draws: 1},
	}
	if diff := cmp.Diff(wantStats, stats); diff != "" {
		t.Fatalf("stats (-want +got):\n%s", diff)
	}
	wantH2H := []HeadToHeadRecord{
		{User: alice, Opponent: bob, Game: "chess", Wins: 1, Draws: 1},
		{User: bob, Opponent: alice, Game: "chess", Losses: 1, Draws: 1},
	}
	if diff := cmp.Diff(wantH2H, h2h); diff != "" {
		t.Fatalf("head-to-head (-want +got):\n%s", diff)
	}

	ids, err := s.FinishedGameIDs(ctx, alice, 10)
	if err != nil || len(ids) != 3 {
		t.Fatalf("finished ids = %v, err = %v", ids, err)
	}
}

func TestPasswordHashNotFound(t *testing.T) {
	s := openTemp(t)
	if _, err := s.PasswordHash(context.Background(), "ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
