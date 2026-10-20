package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
)

var epoch = time.UnixMilli(1_700_000_000_000)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.Context(), filepath.Join(t.TempDir(), "games.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for _, u := range []game.UserID{alice, bob} {
		if err := s.CreateUser(t.Context(), u, []byte("hash")); err != nil {
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
		At:             epoch.Add(time.Duration(seq) * time.Second),
		ClockRemaining: map[game.UserID]time.Duration{alice: time.Duration(seq) * time.Second, bob: 2 * time.Second},
	}
}

var (
	won  = Standing{Outcome: OutcomeWin, Score: 1}
	lost = Standing{Outcome: OutcomeLoss, Score: -1}
	drew = Standing{Outcome: OutcomeDraw, Score: 0}
)

func finishOf(seq game.Seq, kind game.ResultKind, reason game.ResultReason, standings map[game.UserID]Standing) Finish {
	return Finish{
		At: epoch.Add(time.Duration(seq) * time.Second), Seq: seq,
		Ending:  Ending{Kind: kind, Reason: reason, Standings: standings},
		Summary: []byte(`{"game_id":"x"}`), Digest: "d1",
	}
}

func finish(t *testing.T, s *Store, id game.GameID, f Finish) {
	t.Helper()
	if err := s.FinishGame(t.Context(), id, f); err != nil {
		t.Fatal(err)
	}
}

func TestStepsRoundTrip(t *testing.T) {
	ctx := t.Context()
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

func TestLoadOngoingGroupsPlayersAndStepsPerGame(t *testing.T) {
	ctx := t.Context()
	s := openTemp(t)
	first := newGame("g1")
	first.Steps = []StepRecord{step(1, alice), step(2, bob), step(3, alice)}
	second := newGame("g2")
	second.Players = []game.Player{{User: bob, Seat: "white"}, {User: alice, Seat: "black"}}
	second.Steps = []StepRecord{step(1, bob)}
	empty := newGame("g3")
	for _, rec := range []GameRecord{second, first, empty} {
		if err := s.CreateGame(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.LoadOngoing(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]GameRecord{first, second, empty}, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestClockStoredAsMillisecondsOrNull(t *testing.T) {
	ctx := t.Context()
	s := openTemp(t)
	if err := s.CreateGame(ctx, newGame("g1")); err != nil {
		t.Fatal(err)
	}
	untimed := step(2, bob)
	untimed.ClockRemaining = nil
	for _, st := range []StepRecord{step(3, alice), untimed} {
		if err := s.AppendStep(ctx, "g1", st); err != nil {
			t.Fatal(err)
		}
	}
	var timed, none sql.NullString
	if err := s.db.QueryRowContext(ctx,
		`SELECT (SELECT clock_json FROM actions WHERE seq = 3), (SELECT clock_json FROM actions WHERE seq = 2)`).Scan(&timed, &none); err != nil {
		t.Fatal(err)
	}
	if want := `{"alice":3000,"bob":2000}`; timed.String != want || none.Valid {
		t.Fatalf("clock_json = %q and %+v, want %s and NULL", timed.String, none, want)
	}
}

func TestReopenKeepsOngoingGames(t *testing.T) {
	ctx := t.Context()
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
	if err := s.CreateGame(t.Context(), rec); !errors.Is(err, ErrMissingOptions) {
		t.Fatalf("err = %v", err)
	}
}

func TestFinishGameRejectsOutcomeForNonPlayer(t *testing.T) {
	ctx := t.Context()
	s := openTemp(t)
	if err := s.CreateGame(ctx, newGame("g1")); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendStep(ctx, "g1", step(1, alice)); err != nil {
		t.Fatal(err)
	}
	err := s.FinishGame(ctx, "g1", finishOf(2, game.ResultDecisive, game.ReasonResign, map[game.UserID]Standing{"mallory": won}))
	if err == nil {
		t.Fatal("want error")
	}
	if ongoing, _ := s.LoadOngoing(ctx); len(ongoing) != 1 || len(ongoing[0].Steps) != 1 {
		t.Fatal("finish was not rolled back")
	}
}

func TestFinishGameStoresEndingAndDeletesSteps(t *testing.T) {
	ctx := t.Context()
	s := openTemp(t)
	if err := s.CreateGame(ctx, newGame("g1")); err != nil {
		t.Fatal(err)
	}
	for seq := game.Seq(1); seq <= 3; seq++ {
		if err := s.AppendStep(ctx, "g1", step(seq, alice)); err != nil {
			t.Fatal(err)
		}
	}
	want := finishOf(4, game.ResultDecisive, game.ReasonResign, map[game.UserID]Standing{alice: won, bob: lost})
	if err := s.FinishGame(ctx, "g1", want); err != nil {
		t.Fatal(err)
	}

	ongoing, err := s.LoadOngoing(ctx)
	if err != nil || len(ongoing) != 0 {
		t.Fatalf("ongoing = %v, err = %v", ongoing, err)
	}
	rec, err := s.LoadGame(ctx, "g1")
	if err != nil || len(rec.Steps) != 0 {
		t.Fatalf("steps = %d, err = %v; want the finish to delete them", len(rec.Steps), err)
	}
	if diff := cmp.Diff(&want, rec.Finish); diff != "" {
		t.Fatalf("finish (-stored +loaded):\n%s", diff)
	}
	if digest, err := s.Digest(ctx, "g1"); err != nil || digest != want.Digest {
		t.Fatalf("digest = %q, %v", digest, err)
	}
}

func TestFinishGameRefusesSecondFinish(t *testing.T) {
	ctx := t.Context()
	s := openTemp(t)
	if err := s.CreateGame(ctx, newGame("g1")); err != nil {
		t.Fatal(err)
	}
	finish(t, s, "g1", finishOf(1, game.ResultDecisive, game.ReasonResign, map[game.UserID]Standing{alice: won, bob: lost}))
	again := finishOf(2, game.ResultDraw, game.ReasonDrawAgreed, map[game.UserID]Standing{alice: drew, bob: drew})
	again.Digest = "d2"
	if err := s.FinishGame(ctx, "g1", again); !errors.Is(err, ErrNotOngoing) {
		t.Fatalf("second finish: want ErrNotOngoing, got %v", err)
	}
	rec, err := s.LoadGame(ctx, "g1")
	if err != nil || rec.Finish.Digest != "d1" || rec.Finish.Standings[alice] != won {
		t.Fatalf("after a refused second finish: %+v, %v", rec.Finish, err)
	}
}

func TestLoadFinishedKeepsPlayersWhenTopNShiftsMidLoad(t *testing.T) {
	ctx := t.Context()
	s := openTemp(t)
	for _, id := range []game.GameID{"g1", "g2", "g3"} {
		if err := s.CreateGame(ctx, newGame(id)); err != nil {
			t.Fatal(err)
		}
	}
	standings := map[game.UserID]Standing{alice: won, bob: lost}
	finish(t, s, "g1", finishOf(1, game.ResultDecisive, game.ReasonResign, standings))
	finish(t, s, "g2", finishOf(2, game.ResultDecisive, game.ReasonResign, standings))
	shifted := false
	s.afterResolve = func() {
		if !shifted {
			shifted = true
			finish(t, s, "g3", finishOf(3, game.ResultDecisive, game.ReasonResign, standings))
		}
	}

	recs, err := s.LoadFinished(ctx, alice, 2)
	if err != nil || !shifted {
		t.Fatalf("load = %v, shifted = %t", err, shifted)
	}
	for _, rec := range recs {
		if len(rec.Players) != 2 || len(rec.Finish.Standings) != 2 {
			t.Errorf("%s: players %v, standings %v; want both", rec.ID, rec.Players, rec.Finish.Standings)
		}
	}
	ids := make([]game.GameID, 0, len(recs))
	for _, rec := range recs {
		ids = append(ids, rec.ID)
	}
	if !slices.Equal(ids, []game.GameID{"g2", "g1"}) {
		t.Errorf("ids %v, want those resolved before the shift, newest first", ids)
	}
}

func TestStatsExcludeAborted(t *testing.T) {
	ctx := t.Context()
	s := openTemp(t)
	for _, id := range []game.GameID{"win", "draw", "abort"} {
		if err := s.CreateGame(ctx, newGame(id)); err != nil {
			t.Fatal(err)
		}
	}
	finish(t, s, "win", finishOf(1, game.ResultDecisive, game.ReasonResign, map[game.UserID]Standing{alice: won, bob: lost}))
	finish(t, s, "draw", finishOf(1, game.ResultDraw, game.ReasonDrawAgreed, map[game.UserID]Standing{alice: drew, bob: drew}))
	finish(t, s, "abort", finishOf(1, game.ResultAborted, game.ReasonAborted, nil))

	stats, h2h, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantStats := []StatRecord{
		{User: alice, Game: "chess", Tally: Tally{Wins: 1, Draws: 1}},
		{User: bob, Game: "chess", Tally: Tally{Losses: 1, Draws: 1}},
	}
	if diff := cmp.Diff(wantStats, stats); diff != "" {
		t.Fatalf("stats (-want +got):\n%s", diff)
	}
	wantH2H := []HeadToHeadRecord{
		{User: alice, Opponent: bob, Game: "chess", Tally: Tally{Wins: 1, Draws: 1}},
		{User: bob, Opponent: alice, Game: "chess", Tally: Tally{Losses: 1, Draws: 1}},
	}
	if diff := cmp.Diff(wantH2H, h2h); diff != "" {
		t.Fatalf("head-to-head (-want +got):\n%s", diff)
	}

	finished, err := s.LoadFinished(ctx, alice, 10)
	if err != nil || len(finished) != 3 {
		t.Fatalf("finished = %v, err = %v", finished, err)
	}
}

func TestPasswordHashNotFound(t *testing.T) {
	s := openTemp(t)
	if _, err := s.PasswordHash(t.Context(), "ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
