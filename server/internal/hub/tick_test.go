package hub_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/prototest"
)

const metronomeType game.GameType = "metronome"

func metronomeOf(t *testing.T, u protocol.GameUpdate) gametest.MetronomeView {
	t.Helper()
	var view gametest.MetronomeView
	if err := json.Unmarshal(u.View, &view); err != nil {
		t.Fatal(err)
	}
	return view
}

func snapshotOf(t *testing.T, c *prototest.Client, id game.GameID) protocol.GameUpdate {
	t.Helper()
	return call[protocol.GameUpdate](t, c, protocol.TypeGameSnapshot, protocol.GameSnapshotRequest{GameID: id})
}

// pausedSnapshot waits for the game to be paused, as a disconnect does from the session's own goroutine.
func pausedSnapshot(t *testing.T, c *prototest.Client, id game.GameID) protocol.GameUpdate {
	t.Helper()
	deadline := time.Now().Add(pushTimeout)
	for {
		u := snapshotOf(t, c, id)
		if metronomeOf(t, u).Paused {
			return u
		}
		if time.Now().After(deadline) {
			t.Fatalf("game %s not paused: %+v", id, u)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestTickingGamePausesWhenItsPlayerLeavesAndOnRestart(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	srv := e.start(t)
	a := srv.login(t, alice)
	id := call[protocol.GameUpdate](t, a, protocol.TypeGameStart, protocol.GameStartRequest{Game: metronomeType}).GameID
	e.clock.Advance(gametest.MetronomeInterval)
	if u := waitUpdate(t, a, 1); metronomeOf(t, u).Ticks != 1 || len(u.Events) != 1 {
		t.Fatalf("tick update %+v, want one tick and its beat", u)
	}
	again := srv.login(t, alice)
	_ = a.Close()
	time.Sleep(10 * time.Millisecond)
	if u := snapshotOf(t, again, id); metronomeOf(t, u).Paused {
		t.Fatalf("paused while alice still has a session: %+v", u)
	}
	_ = again.Close()

	b := srv.login(t, alice)
	paused := pausedSnapshot(t, b, id)
	e.clock.Advance(time.Hour)
	if u := snapshotOf(t, b, id); u.Seq != paused.Seq {
		t.Fatalf("a paused game stepped to seq %d from %d", u.Seq, paused.Seq)
	}
	resumed := call[protocol.SeqResult](t, b, protocol.TypeGameAct, protocol.GameActRequest{
		GameID: id, Seq: paused.Seq, Action: json.RawMessage(`{"kind":"resume"}`),
	}).Seq
	srv.stop()

	c := e.start(t).login(t, alice)
	if u := snapshotOf(t, c, id); u.Seq != resumed+1 || !metronomeOf(t, u).Paused || metronomeOf(t, u).Ticks != 1 {
		t.Fatalf("after a restart %+v, want seq %d paused after one tick", u, resumed+1)
	}
}

func TestADisconnectPauseArrivingAfterTheUserIsBackIsDropped(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	srv := e.start(t)
	gone := srv.login(t, alice)
	id := call[protocol.GameUpdate](t, gone, protocol.TypeGameStart, protocol.GameStartRequest{Game: metronomeType}).GameID
	_ = gone.Close()
	back := srv.login(t, alice)
	paused := pausedSnapshot(t, back, id)
	call[protocol.SeqResult](t, back, protocol.TypeGameAct, protocol.GameActRequest{
		GameID: id, Seq: paused.Seq, Action: json.RawMessage(`{"kind":"resume"}`),
	})

	srv.hub.PauseGamesOf(alice)
	if u := snapshotOf(t, back, id); metronomeOf(t, u).Paused {
		t.Fatalf("a late disconnect pause stopped the game alice resumed: %+v", u)
	}
}
