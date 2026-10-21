package hub_test

import (
	"context"
	"encoding/json"
	"net"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/games"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
	"github.com/shivamshivanshu/games.nvim/server/internal/auth"
	"github.com/shivamshivanshu/games.nvim/server/internal/clocktest"
	"github.com/shivamshivanshu/games.nvim/server/internal/hub"
	"github.com/shivamshivanshu/games.nvim/server/internal/limits"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/session"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
	"github.com/shivamshivanshu/games.nvim/server/internal/storetest"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

const (
	alice         game.UserID   = "alice"
	bob           game.UserID   = "bob"
	carol         game.UserID   = "carol"
	dave          game.UserID   = "dave"
	testPassword                = "correct horse"
	chessType     game.GameType = "chess"
	shutdownWait                = 5 * time.Second
	moveThinkTime               = 7 * time.Second
	lockedOutText               = "locked out, retry later"
)

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

var passwordHash = sync.OnceValues(func() ([]byte, error) { return auth.HashPassword(testPassword) })

type env struct {
	store *store.Store
	clock *clocktest.FakeClock
}

func newEnv(t *testing.T) *env {
	t.Helper()
	hash, err := passwordHash()
	if err != nil {
		t.Fatal(err)
	}
	return &env{store: storetest.Open(t, hash, alice, bob, carol, dave), clock: clocktest.New(clocktest.Epoch)}
}

type server struct {
	addr string
	stop func()
	hub  *hub.Hub
}

func (e *env) start(t *testing.T) server {
	t.Helper()
	lim := limits.Default()
	ctx, cancel := context.WithCancel(context.Background())
	h, err := hub.New(ctx, hub.Deps{
		Store:   e.store,
		Games:   append(games.All(), game.Adapt(gametest.CounterRules{}), game.Adapt(gametest.ScoredCounterRules{}), game.Adapt(gametest.SoloCounterRules{}), game.Adapt(gametest.MetronomeRules{})),
		Limits:  lim,
		Time:    e.clock,
		Lockout: auth.NewLockout(lim, e.clock),
	})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() { served <- session.Serve(ctx, ln, lim, h.Login(), h.Routes(), h.Hooks()) }()
	stop := sync.OnceFunc(func() {
		cancel()
		select {
		case err := <-served:
			if err != nil {
				t.Errorf("serve: %v", err)
			}
		case <-time.After(shutdownWait):
			t.Error("serve did not return after shutdown")
		}
	})
	t.Cleanup(stop)
	return server{addr: ln.Addr().String(), stop: stop, hub: h}
}

func (s server) login(t *testing.T, user game.UserID) *testClient {
	t.Helper()
	c := dial(t, s.addr)
	call[protocol.LoginResult](t, c, protocol.TypeAuthLogin, protocol.LoginRequest{V: protocol.Version, Username: user, Password: testPassword})
	return c
}

type chessOptions struct {
	TimeControl game.TimeControl  `json:"time_control,omitempty"`
	Color       chess.ColorChoice `json:"color,omitempty"`
}

func startGame(t *testing.T, challenger, opponent *testClient, opts chessOptions) game.GameID {
	t.Helper()
	rawOpts, err := json.Marshal(opts)
	if err != nil {
		t.Fatal(err)
	}
	return startGameOf(t, challenger, opponent, bob, chessType, rawOpts)
}

func startGameOf(t *testing.T, challenger, opponent *testClient, to game.UserID, typ game.GameType, rawOpts json.RawMessage) game.GameID {
	t.Helper()
	created := call[protocol.ChallengeCreateResult](t, challenger, protocol.TypeChallengeCreate,
		protocol.ChallengeCreateRequest{To: []game.UserID{to}, Game: typ, Options: rawOpts})
	for {
		invite := waitPush[protocol.Challenge](t, opponent, protocol.TypeChallengeChanged)
		if invite.ChallengeID == created.ChallengeID && invite.State == protocol.ChallengePending {
			break
		}
	}
	call[protocol.Empty](t, opponent, protocol.TypeChallengeAccept, protocol.ChallengeRef(created))
	first := waitUpdate(t, challenger, 0)
	return first.GameID
}

func waitUpdate(t *testing.T, c *testClient, seq game.Seq) protocol.GameUpdate {
	t.Helper()
	for {
		u := waitPush[protocol.GameUpdate](t, c, protocol.TypeGameUpdate)
		if u.Seq == seq {
			return u
		}
		if u.Seq > seq {
			t.Fatalf("got update seq %d while waiting for %d", u.Seq, seq)
		}
	}
}

func move(t *testing.T, c *testClient, id game.GameID, seq game.Seq, san string) game.Seq {
	t.Helper()
	action, err := json.Marshal(chess.Action{Kind: chess.ActionMove, Move: san})
	if err != nil {
		t.Fatal(err)
	}
	return call[protocol.SeqResult](t, c, protocol.TypeGameAct, protocol.GameActRequest{GameID: id, Seq: seq, Action: action}).Seq
}

func ongoing(t *testing.T, c *testClient) []protocol.GameUpdate {
	t.Helper()
	return call[protocol.GameListResult](t, c, protocol.TypeGameList, protocol.GameListRequest{}).Games
}

func TestFoolsMateEndToEnd(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	srv := e.start(t)
	a, b := srv.login(t, alice), srv.login(t, bob)

	id := startGame(t, a, b, chessOptions{TimeControl: game.TimeControlNone, Color: chess.ColorWhite})
	if u := waitUpdate(t, b, 0); u.GameID != id {
		t.Fatalf("bob's first update is for %s, want %s", u.GameID, id)
	}

	var seq game.Seq
	for i, san := range []string{"f3", "e5", "g4", "Qh4#"} {
		player := a
		if i%2 == 1 {
			player = b
		}
		seq = move(t, player, id, seq, san)
	}

	var finishing protocol.GameUpdate
	for _, c := range []*testClient{a, b} {
		finishing = waitUpdate(t, c, seq)
		if finishing.Result == nil || finishing.Result.Reason != chess.ReasonCheckmate || !slices.Equal(finishing.Result.Winners, []game.UserID{bob}) {
			t.Fatalf("final result = %+v, want checkmate won by bob", finishing.Result)
		}
		e.requireSealed(t, finishing)
		if !strings.Contains(string(finishing.View), "Qh4#") {
			t.Fatalf("finishing broadcast view %s lacks the mating move", finishing.View)
		}
		if pgn := finishing.Exports[chess.PGNFile]; !strings.Contains(pgn, "1. f3 e5 2. g4 Qh4# 0-1") {
			t.Fatalf("finishing broadcast exports %v, want the PGN", finishing.Exports)
		}
	}

	stats := call[protocol.StatsResult](t, a, protocol.TypeStatsGet, protocol.Empty{})
	want := []protocol.StatsRecord{
		{User: bob, Game: chessType, Tally: protocol.Tally{Wins: 1}},
		{User: alice, Game: chessType, Tally: protocol.Tally{Losses: 1}},
	}
	if diff := cmp.Diff(want, stats.Records); diff != "" {
		t.Fatalf("stats records (-want +got):\n%s", diff)
	}
	wantLeaderboard := []protocol.LeaderboardEntry{{User: bob, Tally: protocol.Tally{Wins: 1}}, {User: alice, Tally: protocol.Tally{Losses: 1}}}
	if diff := cmp.Diff(wantLeaderboard, stats.Leaderboard); diff != "" {
		t.Fatalf("leaderboard (-want +got):\n%s", diff)
	}
	if own := call[protocol.StatsResult](t, a, protocol.TypeStatsGet, protocol.StatsRequest{User: alice}); len(own.Records) != 1 || own.Records[0].User != alice {
		t.Fatalf("alice's stats records %+v, want hers alone", own.Records)
	}
	wantHeadToHead := []protocol.HeadToHeadRecord{
		{User: alice, Opponent: bob, Game: chessType, Tally: protocol.Tally{Losses: 1}},
		{User: bob, Opponent: alice, Game: chessType, Tally: protocol.Tally{Wins: 1}},
	}
	if diff := cmp.Diff(wantHeadToHead, stats.HeadToHead); diff != "" {
		t.Fatalf("head to head (-want +got):\n%s", diff)
	}

	finished := call[protocol.GameListResult](t, a, protocol.TypeGameList, protocol.GameListRequest{Status: protocol.StatusFinished}).Games
	if len(finished) != 1 || finished[0].GameID != id || finished[0].Seq != seq || finished[0].Result == nil {
		t.Fatalf("finished games = %+v, want the checkmated game", finished)
	}
	if string(finished[0].View) != string(finishing.View) || !cmp.Equal(finished[0].Exports, finishing.Exports) {
		t.Fatalf("finished read view %s, exports %v; want the finishing broadcast's", finished[0].View, finished[0].Exports)
	}
	if games := ongoing(t, a); len(games) != 0 {
		t.Fatalf("ongoing games after mate = %+v", games)
	}
}

func TestSameUserTwoConnections(t *testing.T) {
	t.Parallel()
	srv := newEnv(t).start(t)
	first, second, b := srv.login(t, alice), srv.login(t, alice), srv.login(t, bob)

	id := startGame(t, first, b, chessOptions{Color: chess.ColorWhite})
	seq := move(t, first, id, 0, "e4")

	u := waitUpdate(t, second, seq)
	if u.GameID != id || !slices.Equal(u.ToAct, []game.UserID{bob}) {
		t.Fatalf("second connection got %+v", u)
	}
}

func TestRestartRestoresGame(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	srv := e.start(t)
	a, b := srv.login(t, alice), srv.login(t, bob)
	id := startGame(t, a, b, chessOptions{TimeControl: game.TimeControl10Plus5, Color: chess.ColorWhite})
	if !uuidV4.MatchString(string(id)) {
		t.Fatalf("game id %q is not a UUIDv4", id)
	}
	var seq game.Seq
	for i, san := range []string{"e4", "e5", "Nf3", "Nc6"} {
		player := a
		if i%2 == 1 {
			player = b
		}
		if i > 0 {
			e.clock.Advance(moveThinkTime)
		}
		seq = move(t, player, id, seq, san)
	}

	before := ongoing(t, a)
	if len(before) != 1 || before[0].Clock == nil || before[0].Clock.Running == nil {
		t.Fatalf("before restart = %+v, want one game with a running clock", before)
	}
	charged := map[game.UserID]int64{alice: 598_000, bob: 598_000}
	if diff := cmp.Diff(charged, before[0].Clock.RemainingMS); diff != "" {
		t.Fatalf("clocks before restart (-want +got):\n%s", diff)
	}
	srv.stop()
	waitDisarmed(t, e.clock)
	e.clock.Advance(time.Hour)

	restarted := e.start(t)
	a = restarted.login(t, alice)
	after := ongoing(t, a)
	if diff := cmp.Diff(before, after, cmpopts.IgnoreFields(protocol.WireClock{}, "AsOfMS")); diff != "" {
		t.Fatalf("game.list across restart (-before +after):\n%s", diff)
	}
	if next := move(t, a, id, seq, "Bc4"); next != seq+1 {
		t.Fatalf("move after restart returned seq %d, want %d", next, seq+1)
	}
}

func TestTimeoutListsAsFinishedAfterRestart(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	srv := e.start(t)
	a, b := srv.login(t, alice), srv.login(t, bob)
	id := startGame(t, a, b, chessOptions{TimeControl: game.TimeControl10Plus5, Color: chess.ColorWhite})
	seq := move(t, b, id, move(t, a, id, 0, "e4"), "e5")

	e.clock.Advance(10 * time.Minute)
	final := waitUpdate(t, a, seq+1)
	if final.Result == nil || final.Result.Reason != game.ReasonTimeout || !slices.Equal(final.Result.Winners, []game.UserID{bob}) {
		t.Fatalf("final result = %+v, want timeout won by bob", final.Result)
	}
	srv.stop()
	waitDisarmed(t, e.clock)

	a = e.start(t).login(t, alice)
	if games := ongoing(t, a); len(games) != 0 {
		t.Fatalf("ongoing after restart = %+v, want none", games)
	}
	finished := call[protocol.GameListResult](t, a, protocol.TypeGameList, protocol.GameListRequest{Status: protocol.StatusFinished}).Games
	if diff := cmp.Diff([]protocol.GameUpdate{laterRead(final)}, finished); diff != "" {
		t.Fatalf("finished list after restart (-want +listed):\n%s", diff)
	}
}

// waitDisarmed waits for the stopped hub's rooms to exit, which stops their
// flag timers, so advancing the clock cannot fire a timeout in a dying room.
func waitDisarmed(t *testing.T, clock *clocktest.FakeClock) {
	t.Helper()
	deadline := time.Now().Add(shutdownWait)
	for len(clock.Armed()) > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("timers still armed after shutdown: %v", clock.Armed())
		}
		time.Sleep(time.Millisecond)
	}
}

func TestOngoingListsYourTurnFirstThenMostRecent(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	srv := e.start(t)
	a, b := srv.login(t, alice), srv.login(t, bob)
	untimed := chessOptions{TimeControl: game.TimeControlNone}

	untimed.Color = chess.ColorWhite
	alicesTurn := startGame(t, a, b, untimed)
	e.clock.Advance(time.Second)
	untimed.Color = chess.ColorBlack
	bobsTurn := startGame(t, a, b, untimed)

	if got := gameIDs(ongoing(t, a)); !slices.Equal(got, []game.GameID{alicesTurn, bobsTurn}) {
		t.Fatalf("order = %v, want alice's turn first", got)
	}
	e.clock.Advance(time.Second)
	move(t, b, bobsTurn, 0, "e4")
	if got := gameIDs(ongoing(t, a)); !slices.Equal(got, []game.GameID{bobsTurn, alicesTurn}) {
		t.Fatalf("order = %v, want most recently moved first", got)
	}
}

func gameIDs(games []protocol.GameUpdate) []game.GameID {
	return xslices.Map(games, func(g protocol.GameUpdate) game.GameID { return g.GameID })
}

func TestChallengeUnknownUser(t *testing.T) {
	t.Parallel()
	a := newEnv(t).start(t).login(t, alice)
	_, err := a.Request(protocol.TypeChallengeCreate, protocol.ChallengeCreateRequest{To: []game.UserID{"mallory"}, Game: chessType})
	wantCode(t, err, protocol.CodeBadRequest)
}

func TestUnknownGameIsNotFound(t *testing.T) {
	t.Parallel()
	a := newEnv(t).start(t).login(t, alice)
	_, err := a.Request(protocol.TypeGameSnapshot, protocol.GameSnapshotRequest{GameID: "nope"})
	wantCode(t, err, protocol.CodeNotFound)
	_, err = a.Request(protocol.TypeGameAct, protocol.GameActRequest{GameID: "nope"})
	wantCode(t, err, protocol.CodeNotFound)
}

func TestWrongPasswordAndLockout(t *testing.T) {
	t.Parallel()
	c := dial(t, newEnv(t).start(t).addr)
	attempt := func(user game.UserID, password string) error {
		_, err := c.Request(protocol.TypeAuthLogin, protocol.LoginRequest{V: protocol.Version, Username: user, Password: password})
		return err
	}

	unknownUser := wantCode(t, attempt("mallory", testPassword), protocol.CodeAuthFailed)
	wrongPassword := wantCode(t, attempt(alice, "wrong"), protocol.CodeAuthFailed)
	if unknownUser.Message != wrongPassword.Message {
		t.Fatalf("unknown user says %q, wrong password says %q", unknownUser.Message, wrongPassword.Message)
	}
	for range limits.Default().LockoutFailures - 2 {
		wantCode(t, attempt(alice, "wrong"), protocol.CodeAuthFailed)
	}
	locked := wantCode(t, attempt(alice, testPassword), protocol.CodeAuthFailed)
	if !strings.Contains(locked.Message, lockedOutText) {
		t.Fatalf("correct password while locked says %q, want %q", locked.Message, lockedOutText)
	}
}

func TestStaleSeq(t *testing.T) {
	t.Parallel()
	srv := newEnv(t).start(t)
	a, b := srv.login(t, alice), srv.login(t, bob)
	id := startGame(t, a, b, chessOptions{Color: chess.ColorWhite})
	move(t, a, id, 0, "e4")

	action, err := json.Marshal(chess.Action{Kind: chess.ActionMove, Move: "e5"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.Request(protocol.TypeGameAct, protocol.GameActRequest{GameID: id, Seq: 0, Action: action})
	wantCode(t, err, protocol.CodeStale)
}

func TestFreshUserListsEncodeEmptyArrays(t *testing.T) {
	t.Parallel()
	a := newEnv(t).start(t).login(t, alice)
	for _, tc := range []struct {
		typ  protocol.MessageType
		body any
		want string
	}{
		{protocol.TypeGameList, protocol.GameListRequest{}, `{"games":[]}`},
		{protocol.TypeGameList, protocol.GameListRequest{Status: protocol.StatusFinished}, `{"games":[]}`},
		{protocol.TypeChallengeList, protocol.Empty{}, `{"challenges":[]}`},
		{protocol.TypeStatsGet, protocol.Empty{}, `{"records":[],"head_to_head":[],"leaderboard":[]}`},
	} {
		raw, err := a.Request(tc.typ, tc.body)
		if err != nil {
			t.Fatalf("%s: %v", tc.typ, err)
		}
		if string(raw) != tc.want {
			t.Errorf("%s %+v = %s, want %s", tc.typ, tc.body, raw, tc.want)
		}
	}
}
