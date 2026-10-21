package hub_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/archive"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/prototest"
)

const (
	counterType       game.GameType = "counter"
	scoredCounterType game.GameType = "scored_counter"
)

func add(t *testing.T, c *prototest.Client, id game.GameID, seq game.Seq, n int) game.Seq {
	t.Helper()
	action := json.RawMessage(fmt.Sprintf(`{"Add":%d}`, n))
	return call[protocol.SeqResult](t, c, protocol.TypeGameAct, protocol.GameActRequest{GameID: id, Seq: seq, Action: action}).Seq
}

// playCounter has the challenger add 5, the opponent 1, the challenger 5: the
// challenger reaches the target and wins at seq 3.
func playCounter(t *testing.T, typ game.GameType, challenger, opponent *prototest.Client, to game.UserID) game.GameID {
	t.Helper()
	id := startGameOf(t, challenger, opponent, to, typ, json.RawMessage(`{}`))
	add(t, challenger, id, add(t, opponent, id, add(t, challenger, id, 0, 5), 1), 5)
	return id
}

// requireSealed checks a finished update offers no further play and carries
// the summary whose digest the store holds, and that the game's steps are gone.
func (e *env) requireSealed(t *testing.T, u protocol.GameUpdate) {
	t.Helper()
	if u.Controls == nil || len(u.Controls) != 0 || u.ToAct == nil || len(u.ToAct) != 0 ||
		u.Clock != nil && u.Clock.Running != nil || u.FinishedAt == nil {
		t.Fatalf("finished update still offers play: %+v", u)
	}
	if u.Summary == nil || u.Digest == nil || archive.Sum([]byte(*u.Summary)) != *u.Digest {
		t.Fatalf("finished update summary %v does not hash to its digest %v", u.Summary, u.Digest)
	}
	rec, err := e.store.LoadGame(context.Background(), u.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Finish == nil || rec.Finish.Digest != *u.Digest || len(rec.Steps) != 0 {
		t.Fatalf("stored game %s: finish %+v, %d steps; want digest %s and no steps", u.GameID, rec.Finish, len(rec.Steps), *u.Digest)
	}
}

// laterRead is how every read after the finishing broadcast shows the game:
// its final view, without the last step's events or the clock.
func laterRead(finishing protocol.GameUpdate) protocol.GameUpdate {
	u := finishing
	u.Events, u.Clock = []json.RawMessage{}, nil
	return u
}

func summaryOutcomes(t *testing.T, u protocol.GameUpdate) []archive.Standing {
	t.Helper()
	var s archive.Summary
	if err := json.Unmarshal([]byte(*u.Summary), &s); err != nil {
		t.Fatal(err)
	}
	return s.Outcomes
}

func TestFinishSealsCanonicalSummaryAndDeletesSteps(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	srv := e.start(t)
	a, b := srv.login(t, alice), srv.login(t, bob)
	id := playCounter(t, counterType, b, a, alice)

	finals := map[game.UserID]protocol.GameUpdate{alice: waitUpdate(t, a, 3), bob: waitUpdate(t, b, 3)}
	if diff := cmp.Diff(finals[alice], finals[bob]); diff != "" {
		t.Fatalf("final updates differ (-alice +bob):\n%s", diff)
	}
	final := finals[alice]
	e.requireSealed(t, final)
	at := e.clock.Now().UnixMilli()
	want := fmt.Sprintf(`{"game_id":%q,"game":"counter","started_at":%d,"finished_at":%d,"reason":"target_reached",`+
		`"outcomes":[{"user":"alice","score":-1},{"user":"bob","score":1}]}`, id, at, at)
	if *final.Summary != want {
		t.Fatalf("summary\n got %s\nwant %s", *final.Summary, want)
	}
	wantResult := &protocol.WireResult{Kind: game.ResultDecisive, Winners: []game.UserID{bob}, Reason: "target_reached"}
	if diff := cmp.Diff(wantResult, final.Result); diff != "" {
		t.Errorf("final result (-want +got):\n%s", diff)
	}
	for user, c := range map[game.UserID]*prototest.Client{alice: a, bob: b} {
		listed := call[protocol.GameListResult](t, c, protocol.TypeGameList, protocol.GameListRequest{Status: protocol.StatusFinished}).Games
		if diff := cmp.Diff([]protocol.GameUpdate{laterRead(final)}, listed); diff != "" {
			t.Errorf("%s's finished list (-want +listed):\n%s", user, diff)
		}
	}
}

func TestFinishingBroadcastAddsLastStepToEveryLaterRead(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	srv := e.start(t)
	a, b := srv.login(t, alice), srv.login(t, bob)
	id := playCounter(t, counterType, a, b, bob)

	for user, c := range map[game.UserID]*prototest.Client{alice: a, bob: b} {
		finishing := waitUpdate(t, c, 3)
		e.requireSealed(t, finishing)
		events := make([]string, 0, len(finishing.Events))
		for _, ev := range finishing.Events {
			events = append(events, string(ev))
		}
		if string(finishing.View) != `{"Total":11}` || !cmp.Equal(events, []string{`{"by":"a","kind":"counted","total":11}`}) {
			t.Fatalf("%s's finishing broadcast view %s, events %v; want the final total and its counted event", user, finishing.View, events)
		}
		snapshot := call[protocol.GameUpdate](t, c, protocol.TypeGameSnapshot, protocol.GameSnapshotRequest{GameID: id})
		if diff := cmp.Diff(laterRead(finishing), snapshot); diff != "" {
			t.Errorf("%s's later snapshot (-want +got):\n%s", user, diff)
		}
	}
}

func TestVerifyAcceptsOnlyTheStoredSummary(t *testing.T) {
	t.Parallel()
	srv := newEnv(t).start(t)
	a, b, c := srv.login(t, alice), srv.login(t, bob), srv.login(t, carol)
	id := playCounter(t, counterType, a, b, bob)
	summary := *waitUpdate(t, a, 3).Summary
	verify := func(summary string) error {
		_, err := c.Request(protocol.TypeGameVerify, protocol.GameVerifyRequest{Summary: summary})
		return err
	}

	if err := verify(summary); err != nil {
		t.Fatalf("original summary: %v", err)
	}
	flipped := strings.Replace(summary, `"score":1}`, `"score":2}`, 1)
	if flipped == summary {
		t.Fatalf("test setup: no score to change in %s", summary)
	}
	ongoing := startGameOf(t, srv.login(t, alice), srv.login(t, bob), bob, counterType, json.RawMessage(`{}`))
	for name, bad := range map[string]string{
		"one byte changed": flipped,
		"truncated":        summary[:len(summary)-1],
		"unknown game":     strings.Replace(summary, string(id), "00000000-0000-4000-8000-000000000000", 1),
		"ongoing game":     strings.Replace(summary, string(id), string(ongoing), 1),
	} {
		err := verify(bad)
		if e, ok := errors.AsType[protocol.Error](err); !ok || e.Code != protocol.CodeSummaryMismatch {
			t.Errorf("%s: want %s, got %v", name, protocol.CodeSummaryMismatch, err)
		}
	}
}

func TestScorerReplacesDefaultScores(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	srv := e.start(t)
	a, b := srv.login(t, alice), srv.login(t, bob)
	playCounter(t, scoredCounterType, a, b, bob)

	final := waitUpdate(t, a, 3)
	e.requireSealed(t, final)
	outcomes := summaryOutcomes(t, final)
	if want := []archive.Standing{{User: alice, Score: 9}, {User: bob, Score: -9}}; !cmp.Equal(want, outcomes) {
		t.Fatalf("scores %+v, want the scorer's zero-sum %+v", outcomes, want)
	}
	stats := call[protocol.StatsResult](t, a, protocol.TypeStatsGet, protocol.Empty{})
	want := []protocol.StatsRecord{
		{User: alice, Game: scoredCounterType, Tally: protocol.Tally{Wins: 1}},
		{User: bob, Game: scoredCounterType, Tally: protocol.Tally{Losses: 1}},
	}
	if diff := cmp.Diff(want, stats.Records); diff != "" {
		t.Fatalf("stats (-want +got):\n%s", diff)
	}
}
