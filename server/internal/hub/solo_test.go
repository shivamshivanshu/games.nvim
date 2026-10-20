package hub_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/internal/archive"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
)

const soloCounterType game.GameType = "solo_counter"

func startSolo(t *testing.T, c *testClient, rawOpts string) protocol.GameUpdate {
	t.Helper()
	return call[protocol.GameUpdate](t, c, protocol.TypeGameStart,
		protocol.GameStartRequest{Game: soloCounterType, Options: json.RawMessage(rawOpts)})
}

func TestSoloStartCreatesOnePlayerGame(t *testing.T) {
	t.Parallel()
	a := newEnv(t).start(t).login(t, alice)

	first := startSolo(t, a, `{"step":"2"}`)
	if !uuidV4.MatchString(string(first.GameID)) || first.Seq != 0 || first.Result != nil ||
		!cmp.Equal(first.Players, []game.Player{{User: alice, Seat: "a"}}) ||
		!cmp.Equal(first.ToAct, []game.UserID{alice}) || !cmp.Equal(first.Options, game.Options{"step": "2", "note": ""}) {
		t.Fatalf("first update = %+v, want alice alone to move", first)
	}
	if diff := cmp.Diff(first, waitUpdate(t, a, 0)); diff != "" {
		t.Errorf("pushed update differs from the reply (-reply +push):\n%s", diff)
	}
	if diff := cmp.Diff([]protocol.GameUpdate{first}, ongoing(t, a)); diff != "" {
		t.Errorf("ongoing games (-want +got):\n%s", diff)
	}
}

func TestSoloStartRejectsBadRequests(t *testing.T) {
	t.Parallel()
	a := newEnv(t).start(t).login(t, alice)
	for name, req := range map[string]protocol.GameStartRequest{
		"two-player counter": {Game: counterType},
		"chess":              {Game: chessType},
		"unknown game":       {Game: "nope"},
		"unknown option":     {Game: soloCounterType, Options: json.RawMessage(`{"bogus":"1"}`)},
		"invalid enum":       {Game: soloCounterType, Options: json.RawMessage(`{"step":"9"}`)},
		"oversized text":     {Game: soloCounterType, Options: json.RawMessage(`{"note":"123456789"}`)},
		"control text":       {Game: soloCounterType, Options: json.RawMessage(`{"note":"a\u0000"}`)},
		"malformed options":  {Game: soloCounterType, Options: json.RawMessage(`["step"]`)},
	} {
		_, err := a.Request(protocol.TypeGameStart, req)
		var reply errorReply
		if !errors.As(err, &reply) || reply.Code != protocol.CodeBadRequest {
			t.Errorf("%s: want %s, got %v", name, protocol.CodeBadRequest, err)
		}
	}
	if games := ongoing(t, a); len(games) != 0 {
		t.Fatalf("rejected starts left games %+v", games)
	}
}

func TestSoloGameSurvivesRestartAndFinishes(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	srv := e.start(t)
	a := srv.login(t, alice)
	id := startSolo(t, a, `{"note":"hi"}`).GameID
	seq := add(t, a, id, 0, 3)
	before := ongoing(t, a)
	srv.stop()

	a = e.start(t).login(t, alice)
	if diff := cmp.Diff(before, ongoing(t, a)); diff != "" {
		t.Fatalf("game.list across restart (-before +after):\n%s", diff)
	}
	if next := add(t, a, id, seq, 3); next != seq+1 {
		t.Fatalf("move after restart returned seq %d, want %d", next, seq+1)
	}
	final := waitUpdate(t, a, add(t, a, id, seq+1, 4))
	e.requireSealed(t, final)
	wantResult := &protocol.WireResult{Kind: game.ResultDecisive, Winners: []game.UserID{alice}, Reason: gametest.ReasonTargetReached}
	if diff := cmp.Diff(wantResult, final.Result); diff != "" {
		t.Errorf("final result (-want +got):\n%s", diff)
	}
}

func TestSoloGiveUpSealsLossWithNoWinner(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a := e.start(t).login(t, alice)
	id := startSolo(t, a, `{}`).GameID
	seq := call[protocol.SeqResult](t, a, protocol.TypeGameControl,
		protocol.GameControlRequest{GameID: id, Seq: 0, Control: protocol.ControlResign}).Seq

	final := waitUpdate(t, a, seq)
	e.requireSealed(t, final)
	wantResult := &protocol.WireResult{Kind: game.ResultDecisive, Winners: []game.UserID{}, Reason: game.ReasonResign}
	if diff := cmp.Diff(wantResult, final.Result); diff != "" {
		t.Errorf("final result (-want +got):\n%s", diff)
	}
	if want := []archive.Outcome{{User: alice, Score: -1}}; !cmp.Equal(want, summaryOutcomes(t, final)) {
		t.Errorf("outcomes %+v, want %+v", summaryOutcomes(t, final), want)
	}
	stats := call[protocol.StatsResult](t, a, protocol.TypeStatsGet, protocol.Empty{})
	if want := []protocol.StatsRecord{{User: alice, Game: soloCounterType, Tally: protocol.Tally{Losses: 1}}}; !cmp.Equal(want, stats.Records) {
		t.Errorf("stats %+v, want the solo loss on its own game's board", stats.Records)
	}
}
