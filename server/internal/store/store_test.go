package store_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/clocktest"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
	"github.com/shivamshivanshu/games.nvim/server/internal/storetest"
)

const (
	alice game.UserID = "alice"
	bob   game.UserID = "bob"
)

var passwordHash = []byte("hash")

func newGame(id game.GameID) store.GameRecord {
	return store.GameRecord{
		ID: id, Type: "chess", Options: json.RawMessage(`{"clock":300}`), Seed: 1<<63 + 7,
		Players:   []game.Player{{User: alice, Seat: "white"}, {User: bob, Seat: "black"}},
		CreatedAt: clocktest.Epoch,
	}
}

func step(seq game.Seq, user game.UserID) store.StepRecord {
	return store.StepRecord{
		Seq: seq, User: user, Kind: store.StepAction, Payload: json.RawMessage(`"e4"`),
		At:             clocktest.Epoch.Add(time.Duration(seq) * time.Second),
		ClockRemaining: map[game.UserID]time.Duration{alice: time.Duration(seq) * time.Second, bob: 2 * time.Second},
	}
}

var (
	won  = store.Standing{Outcome: store.OutcomeWin, Score: 1}
	lost = store.Standing{Outcome: store.OutcomeLoss, Score: -1}
	drew = store.Standing{Outcome: store.OutcomeDraw, Score: 0}
)

func finishOf(seq game.Seq, kind game.ResultKind, reason game.ResultReason, standings map[game.UserID]store.Standing) store.Finish {
	views := make(map[game.UserID]json.RawMessage, len(standings))
	for user := range standings {
		views[user] = json.RawMessage(`{"viewer":"` + user + `"}`)
	}
	return store.Finish{
		At: clocktest.Epoch.Add(time.Duration(seq) * time.Second), Seq: seq,
		Ending:  store.Ending{Kind: kind, Reason: reason, Standings: standings},
		Summary: []byte(`{"game_id":"x"}`), Digest: "d1",
		Views: views, Exports: map[string]string{"game.txt": "final"},
	}
}

func finish(t *testing.T, s *store.Store, id game.GameID, f store.Finish) {
	t.Helper()
	if err := s.FinishGame(t.Context(), id, f); err != nil {
		t.Fatal(err)
	}
}

func TestStepsRoundTrip(t *testing.T) {
	ctx := t.Context()
	s := storetest.Open(t, passwordHash, alice, bob)
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
	control.Kind = store.StepControl
	control.Payload = json.RawMessage(`{"op":"resign"}`)
	timeout := store.StepRecord{Seq: 5, Kind: store.StepTimeout, At: clocktest.Epoch.Add(time.Minute)}
	for _, st := range []store.StepRecord{control, timeout} {
		if err := s.AppendStep(ctx, "g1", st); err != nil {
			t.Fatal(err)
		}
		rec.Steps = append(rec.Steps, st)
	}

	got, err := s.LoadOngoing(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]store.GameRecord{rec}, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadOngoingGroupsPlayersAndStepsPerGame(t *testing.T) {
	ctx := t.Context()
	s := storetest.Open(t, passwordHash, alice, bob)
	first := newGame("g1")
	first.Steps = []store.StepRecord{step(1, alice), step(2, bob), step(3, alice)}
	second := newGame("g2")
	second.Players = []game.Player{{User: bob, Seat: "white"}, {User: alice, Seat: "black"}}
	second.Steps = []store.StepRecord{step(1, bob)}
	empty := newGame("g3")
	for _, rec := range []store.GameRecord{second, first, empty} {
		if err := s.CreateGame(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.LoadOngoing(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]store.GameRecord{first, second, empty}, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestClockStoredAsMillisecondsOrNull(t *testing.T) {
	ctx := t.Context()
	s := storetest.Open(t, passwordHash, alice, bob)
	if err := s.CreateGame(ctx, newGame("g1")); err != nil {
		t.Fatal(err)
	}
	untimed := step(2, bob)
	untimed.ClockRemaining = nil
	for _, st := range []store.StepRecord{step(3, alice), untimed} {
		if err := s.AppendStep(ctx, "g1", st); err != nil {
			t.Fatal(err)
		}
	}
	var timed, none sql.NullString
	if err := s.Reader().QueryRowContext(ctx,
		`SELECT (SELECT clock_json FROM actions WHERE seq = 3), (SELECT clock_json FROM actions WHERE seq = 2)`).Scan(&timed, &none); err != nil {
		t.Fatal(err)
	}
	if want := `{"alice":3000,"bob":2000}`; timed.String != want || none.Valid {
		t.Fatalf("clock_json = %q and %+v, want %s and NULL", timed.String, none, want)
	}
}

func TestCreateUserRefusesNamesWithHiddenCharacters(t *testing.T) {
	s := storetest.Open(t, passwordHash)
	for _, name := range []game.UserID{"", "ev\nil", "a\x00b", "bi\u202edi"} {
		if err := s.CreateUser(t.Context(), name, passwordHash); err == nil {
			t.Errorf("created user %q", name)
		}
	}
	if err := s.CreateUser(t.Context(), "zoë", passwordHash); err != nil {
		t.Errorf("refused a plain name: %v", err)
	}
}

func TestReopenKeepsOngoingGames(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "games.db")
	s, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []game.UserID{alice, bob} {
		if err := s.CreateUser(ctx, u, passwordHash); err != nil {
			t.Fatal(err)
		}
	}
	rec := newGame("g1")
	rec.Steps = []store.StepRecord{step(1, alice), step(2, bob)}
	if err := s.CreateGame(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reopened.Close() }()
	got, err := reopened.LoadOngoing(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]store.GameRecord{rec}, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestCreateGameRejectsEmptyOptions(t *testing.T) {
	s := storetest.Open(t, passwordHash, alice, bob)
	rec := newGame("g1")
	rec.Options = nil
	if err := s.CreateGame(t.Context(), rec); !errors.Is(err, store.ErrMissingOptions) {
		t.Fatalf("err = %v", err)
	}
}

func TestFinishGameRejectsOutcomeForNonPlayer(t *testing.T) {
	ctx := t.Context()
	s := storetest.Open(t, passwordHash, alice, bob)
	if err := s.CreateGame(ctx, newGame("g1")); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendStep(ctx, "g1", step(1, alice)); err != nil {
		t.Fatal(err)
	}
	err := s.FinishGame(ctx, "g1", finishOf(2, game.ResultDecisive, game.ReasonResign, map[game.UserID]store.Standing{"mallory": won}))
	if err == nil {
		t.Fatal("want error")
	}
	if ongoing, _ := s.LoadOngoing(ctx); len(ongoing) != 1 || len(ongoing[0].Steps) != 1 {
		t.Fatal("finish was not rolled back")
	}
}

func TestFinishGameStoresEndingAndDeletesSteps(t *testing.T) {
	ctx := t.Context()
	s := storetest.Open(t, passwordHash, alice, bob)
	if err := s.CreateGame(ctx, newGame("g1")); err != nil {
		t.Fatal(err)
	}
	for seq := game.Seq(1); seq <= 3; seq++ {
		if err := s.AppendStep(ctx, "g1", step(seq, alice)); err != nil {
			t.Fatal(err)
		}
	}
	want := finishOf(4, game.ResultDecisive, game.ReasonResign, map[game.UserID]store.Standing{alice: won, bob: lost})
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
	s := storetest.Open(t, passwordHash, alice, bob)
	if err := s.CreateGame(ctx, newGame("g1")); err != nil {
		t.Fatal(err)
	}
	finish(t, s, "g1", finishOf(1, game.ResultDecisive, game.ReasonResign, map[game.UserID]store.Standing{alice: won, bob: lost}))
	again := finishOf(2, game.ResultDraw, game.ReasonDrawAgreed, map[game.UserID]store.Standing{alice: drew, bob: drew})
	again.Digest = "d2"
	if err := s.FinishGame(ctx, "g1", again); !errors.Is(err, store.ErrNotOngoing) {
		t.Fatalf("second finish: want store.ErrNotOngoing, got %v", err)
	}
	rec, err := s.LoadGame(ctx, "g1")
	if err != nil || rec.Finish.Digest != "d1" || rec.Finish.Standings[alice] != won {
		t.Fatalf("after a refused second finish: %+v, %v", rec.Finish, err)
	}
}

func TestLoadFinishedKeepsPlayersWhenTopNShiftsMidLoad(t *testing.T) {
	ctx := t.Context()
	s := storetest.Open(t, passwordHash, alice, bob)
	for _, id := range []game.GameID{"g1", "g2", "g3"} {
		if err := s.CreateGame(ctx, newGame(id)); err != nil {
			t.Fatal(err)
		}
	}
	standings := map[game.UserID]store.Standing{alice: won, bob: lost}
	finish(t, s, "g1", finishOf(1, game.ResultDecisive, game.ReasonResign, standings))
	finish(t, s, "g2", finishOf(2, game.ResultDecisive, game.ReasonResign, standings))
	shifted := false
	s.SetAfterResolve(func() {
		if !shifted {
			shifted = true
			finish(t, s, "g3", finishOf(3, game.ResultDecisive, game.ReasonResign, standings))
		}
	})

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
	s := storetest.Open(t, passwordHash, alice, bob)
	for _, id := range []game.GameID{"win", "draw", "abort"} {
		if err := s.CreateGame(ctx, newGame(id)); err != nil {
			t.Fatal(err)
		}
	}
	finish(t, s, "win", finishOf(1, game.ResultDecisive, game.ReasonResign, map[game.UserID]store.Standing{alice: won, bob: lost}))
	finish(t, s, "draw", finishOf(1, game.ResultDraw, game.ReasonDrawAgreed, map[game.UserID]store.Standing{alice: drew, bob: drew}))
	finish(t, s, "abort", finishOf(1, game.ResultAborted, game.ReasonAborted, nil))

	stats, err := s.Stats(ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	wantStats := []store.StatRecord{
		{User: alice, Game: "chess", Tally: store.Tally{Wins: 1, Draws: 1}},
		{User: bob, Game: "chess", Tally: store.Tally{Losses: 1, Draws: 1}},
	}
	if diff := cmp.Diff(wantStats, stats.Records); diff != "" {
		t.Fatalf("stats (-want +got):\n%s", diff)
	}
	wantH2H := []store.HeadToHeadRecord{
		{User: alice, Opponent: bob, Game: "chess", Tally: store.Tally{Wins: 1, Draws: 1}},
		{User: bob, Opponent: alice, Game: "chess", Tally: store.Tally{Losses: 1, Draws: 1}},
	}
	if diff := cmp.Diff(wantH2H, stats.HeadToHead); diff != "" {
		t.Fatalf("head-to-head (-want +got):\n%s", diff)
	}

	finished, err := s.LoadFinished(ctx, alice, 10)
	if err != nil || len(finished) != 3 {
		t.Fatalf("finished = %v, err = %v", finished, err)
	}
}

func TestStatsRankAndFilterByUser(t *testing.T) {
	const carol game.UserID = "carol"
	ctx := t.Context()
	s := storetest.Open(t, passwordHash, alice, bob, carol)
	games := []struct {
		id            game.GameID
		winner, loser game.UserID
	}{
		{"g1", carol, bob}, {"g2", carol, bob}, {"g3", alice, bob}, {"g4", alice, bob}, {"g5", bob, alice},
	}
	for _, g := range games {
		rec := newGame(g.id)
		rec.Players = []game.Player{{User: g.winner, Seat: "white"}, {User: g.loser, Seat: "black"}}
		if err := s.CreateGame(ctx, rec); err != nil {
			t.Fatal(err)
		}
		finish(t, s, g.id, finishOf(1, game.ResultDecisive, game.ReasonResign, map[game.UserID]store.Standing{g.winner: won, g.loser: lost}))
	}
	wantLeaderboard := []store.UserTally{
		{User: carol, Tally: store.Tally{Wins: 2}},
		{User: alice, Tally: store.Tally{Wins: 2, Losses: 1}},
		{User: bob, Tally: store.Tally{Wins: 1, Losses: 4}},
	}
	all, err := s.Stats(ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ranked := make([]game.UserID, 0, len(all.Records))
	for _, r := range all.Records {
		ranked = append(ranked, r.User)
	}
	if !slices.Equal(ranked, []game.UserID{carol, alice, bob}) {
		t.Errorf("records ranked %v, want carol, alice, bob", ranked)
	}
	alices, err := s.Stats(ctx, alice, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(alices.Records) != 1 || alices.Records[0].User != alice || len(alices.HeadToHead) != 1 || alices.HeadToHead[0].User != alice {
		t.Errorf("alice's stats %+v, want only her own rows", alices)
	}
	for name, got := range map[string][]store.UserTally{"unfiltered": all.Leaderboard, "filtered": alices.Leaderboard} {
		if diff := cmp.Diff(wantLeaderboard, got); diff != "" {
			t.Errorf("%s leaderboard (-want +got):\n%s", name, diff)
		}
	}
}

func TestStatsRankMoreDrawsFirstAmongEqualWinsAndLosses(t *testing.T) {
	const carol game.UserID = "carol"
	ctx := t.Context()
	s := storetest.Open(t, passwordHash, alice, bob, carol)
	games := []map[game.UserID]store.Standing{
		{alice: won, carol: lost}, {carol: won, alice: lost},
		{bob: won, carol: lost}, {carol: won, bob: lost},
		{bob: drew, carol: drew},
	}
	for i, standings := range games {
		id := game.GameID(fmt.Sprintf("g%d", i))
		rec := newGame(id)
		rec.Players = nil
		for user := range standings {
			rec.Players = append(rec.Players, game.Player{User: user, Seat: game.Seat(user)})
		}
		if err := s.CreateGame(ctx, rec); err != nil {
			t.Fatal(err)
		}
		kind := game.ResultDecisive
		if standings[bob] == drew {
			kind = game.ResultDraw
		}
		finish(t, s, id, finishOf(1, kind, game.ReasonResign, standings))
	}
	stats, err := s.Stats(ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ranked := make([]game.UserID, 0, len(stats.Leaderboard))
	for _, entry := range stats.Leaderboard {
		ranked = append(ranked, entry.User)
	}
	if want := []game.UserID{carol, bob, alice}; !slices.Equal(ranked, want) {
		t.Errorf("leaderboard %v, want %v: bob's draw ranks him above alice's equal wins and losses", ranked, want)
	}
}

func TestStatsLeaderboardLeavesOutUnrankedGames(t *testing.T) {
	const solo game.GameType = "solo"
	ctx := t.Context()
	s := storetest.Open(t, passwordHash, alice, bob)
	duel := newGame("duel")
	alone := newGame("alone")
	alone.Type, alone.Players = solo, []game.Player{{User: alice, Seat: "a"}}
	for _, rec := range []store.GameRecord{duel, alone} {
		if err := s.CreateGame(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	finish(t, s, "duel", finishOf(1, game.ResultDecisive, game.ReasonResign, map[game.UserID]store.Standing{alice: won, bob: lost}))
	finish(t, s, "alone", finishOf(1, game.ResultDecisive, game.ReasonResign, map[game.UserID]store.Standing{alice: lost}))

	stats, err := s.Stats(ctx, "", []game.GameType{solo})
	if err != nil {
		t.Fatal(err)
	}
	wantRecords := []store.StatRecord{
		{User: alice, Game: "chess", Tally: store.Tally{Wins: 1}},
		{User: bob, Game: "chess", Tally: store.Tally{Losses: 1}},
		{User: alice, Game: solo, Tally: store.Tally{Losses: 1}},
	}
	if diff := cmp.Diff(wantRecords, stats.Records); diff != "" {
		t.Errorf("records (-want +got):\n%s", diff)
	}
	wantLeaderboard := []store.UserTally{
		{User: alice, Tally: store.Tally{Wins: 1}},
		{User: bob, Tally: store.Tally{Losses: 1}},
	}
	if diff := cmp.Diff(wantLeaderboard, stats.Leaderboard); diff != "" {
		t.Errorf("leaderboard (-want +got):\n%s", diff)
	}
}

func TestReadInProgressDoesNotBlockAppendStep(t *testing.T) {
	ctx := t.Context()
	s := storetest.Open(t, passwordHash, alice, bob)
	if err := s.CreateGame(ctx, newGame("g1")); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Reader().QueryContext(ctx, `SELECT game_id FROM games`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Fatalf("read returned no row: %v", rows.Err())
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.AppendStep(writeCtx, "g1", step(1, alice)); err != nil {
		t.Fatalf("append while a read is open: %v", err)
	}
}

func TestStoreWithoutFileReadsItsOwnWrites(t *testing.T) {
	for _, path := range []string{":memory:", ""} {
		s := storetest.OpenAt(t, path, passwordHash, alice)
		hash, err := s.PasswordHash(t.Context(), alice)
		if err != nil || string(hash) != string(passwordHash) {
			t.Errorf("path %q: hash %q, %v; want %q", path, hash, err, passwordHash)
		}
	}
}

func TestPasswordHashNotFound(t *testing.T) {
	s := storetest.Open(t, passwordHash, alice, bob)
	if _, err := s.PasswordHash(t.Context(), "ghost"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
