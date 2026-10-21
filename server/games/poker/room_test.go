package poker

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/game/cards"
	"github.com/shivamshivanshu/games.nvim/server/game/gametest"
	"github.com/shivamshivanshu/games.nvim/server/internal/clocktest"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/room"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

// recorder is a room's store and network: it keeps the persisted steps and the last update each user was sent.
type recorder struct {
	mu       sync.Mutex
	steps    []store.StepRecord
	finished *store.Finish
	last     map[game.UserID]protocol.GameUpdate
}

func (r *recorder) AppendSteps(_ context.Context, _ game.GameID, steps ...store.StepRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = append(r.steps, steps...)
	return nil
}

func (r *recorder) FinishGame(_ context.Context, _ game.GameID, f store.Finish) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finished = &f
	return nil
}

func (r *recorder) SendUpdate(to game.UserID, u protocol.GameUpdate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.last == nil {
		r.last = map[game.UserID]protocol.GameUpdate{}
	}
	r.last[to] = u
}

func (r *recorder) persisted() ([]store.StepRecord, *store.Finish) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.steps), r.finished
}

func (r *recorder) lastSent(user game.UserID) protocol.GameUpdate {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last[user]
}

type pokerRoom struct {
	room   *room.Room
	params room.Params
	rec    *recorder
	seq    game.Seq
}

func roomParams(t *testing.T, players []game.UserID, seed game.Seed) (room.Params, game.ErasedState) {
	t.Helper()
	g := game.Adapt(Rules{})
	opts, err := g.ResolveOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	st, seats, err := g.New(opts, players[0], players[1:], game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	seated := make([]game.Player, len(players))
	for i, user := range players {
		seated[i] = game.Player{User: user, Seat: seats[user]}
	}
	start := time.UnixMilli(1_700_000_000_000)
	return room.Params{
		ID: "g1", Game: g, Options: opts, Seed: seed, Challenger: players[0], Players: seated,
		CreatedAt: start, Time: clocktest.New(start),
	}, st
}

func withRecorder(t *testing.T, p room.Params) (room.Setup, *recorder) {
	t.Helper()
	rec := &recorder{}
	p.Persist, p.Broadcast = rec, rec
	setup, err := room.NewSetup(p)
	if err != nil {
		t.Fatal(err)
	}
	return setup, rec
}

func openRoom(t *testing.T, players []game.UserID, seed game.Seed) *pokerRoom {
	t.Helper()
	params, st := roomParams(t, players, seed)
	setup, rec := withRecorder(t, params)
	return &pokerRoom{room: room.Start(t.Context(), setup, st), params: params, rec: rec}
}

func restoreRoom(t *testing.T, params room.Params, steps []store.StepRecord) *pokerRoom {
	t.Helper()
	setup, rec := withRecorder(t, params)
	r, err := room.Restore(t.Context(), setup, steps, func() bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	return &pokerRoom{room: r, params: params, rec: rec, seq: game.Seq(len(steps))}
}

func (r *pokerRoom) snapshot(t *testing.T, user game.UserID) protocol.GameUpdate {
	t.Helper()
	s, err := r.room.Snapshot(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	return s.Update
}

func decodeView(t *testing.T, u protocol.GameUpdate) View {
	t.Helper()
	var v View
	if err := json.Unmarshal(u.View, &v); err != nil {
		t.Fatalf("view %s: %v", u.View, err)
	}
	return v
}

// next is the user to act and the move choose picks for them, or false once the game is over.
func (r *pokerRoom) next(t *testing.T) (game.UserID, json.RawMessage, bool) {
	t.Helper()
	toAct := r.snapshot(t, r.params.Challenger).ToAct
	if len(toAct) == 0 {
		return "", nil, false
	}
	u := r.snapshot(t, toAct[0])
	seat := u.Players[slices.IndexFunc(u.Players, func(p game.Player) bool { return p.User == toAct[0] })].Seat
	action, err := json.Marshal(choose(decodeView(t, u), seat))
	if err != nil {
		t.Fatal(err)
	}
	return toAct[0], action, true
}

func (r *pokerRoom) act(t *testing.T, user game.UserID, action json.RawMessage) {
	t.Helper()
	seq, err := r.room.Act(t.Context(), user, r.seq, action)
	if err != nil {
		t.Fatalf("%s %s at seq %d: %v", user, action, r.seq, err)
	}
	r.seq = seq
}

func (r *pokerRoom) resign(t *testing.T, user game.UserID) {
	t.Helper()
	seq, err := r.room.Control(t.Context(), user, r.seq, protocol.ControlResign)
	if err != nil {
		t.Fatalf("%s resigns: %v", user, err)
	}
	r.seq = seq
}

func TestRestoreReplaysPersistedStepsAndPlaysOnIdentically(t *testing.T) {
	const (
		seed           game.Seed = 11
		before, resign           = 30, 10
	)
	players := gametest.Users(4)
	live := openRoom(t, players, seed)
	for k := range before + resign {
		user, action, ok := live.next(t)
		if !ok {
			t.Fatalf("game finished at step %d, before the restore point", k)
		}
		if k == before {
			live.resign(t, user)
			continue
		}
		live.act(t, user, action)
	}
	steps, _ := live.rec.persisted()
	restored := restoreRoom(t, live.params, steps)

	for {
		for _, user := range players {
			want, got := live.snapshot(t, user), restored.snapshot(t, user)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("seq %d: %s's update differs (-live +restored):\n%s", live.seq, user, diff)
			}
		}
		user, action, ok := live.next(t)
		if !ok {
			break
		}
		live.act(t, user, action)
		restored.act(t, user, action)
	}
	_, liveFinish := live.rec.persisted()
	_, restoredFinish := restored.rec.persisted()
	if liveFinish == nil || restoredFinish == nil {
		t.Fatal("both games must finish")
	}
	if diff := cmp.Diff(liveFinish.Ending, restoredFinish.Ending); diff != "" {
		t.Errorf("endings differ (-live +restored):\n%s", diff)
	}
	var sum int64
	for _, standing := range liveFinish.Standings {
		sum += standing.Score
	}
	if sum != 0 {
		t.Errorf("chip deltas %v sum to %d, want 0", liveFinish.Standings, sum)
	}
}

// unrevealed is what u's own view shows that only u may see: its hole cards,
// less any card a showdown or the board has made public.
func unrevealed(v View) []cards.Card {
	public := slices.Clone(v.Board)
	if v.LastHand != nil {
		public = append(public, v.LastHand.Board...)
		for _, shown := range v.LastHand.Shown {
			public = append(public, shown.Cards...)
		}
	}
	return slices.DeleteFunc(slices.Clone(v.MyCards), func(c cards.Card) bool { return slices.Contains(public, c) })
}

func TestWireUpdateNeverCarriesAnotherSeatsHoleCards(t *testing.T) {
	players := gametest.Users(3)
	r := openRoom(t, players, 4)
	checked := 0
	for {
		user, action, ok := r.next(t)
		if !ok {
			break
		}
		r.act(t, user, action)
		for _, owner := range players {
			private := unrevealed(decodeView(t, r.rec.lastSent(owner)))
			for _, other := range players {
				if other == owner {
					continue
				}
				wire, err := json.Marshal(r.rec.lastSent(other))
				if err != nil {
					t.Fatal(err)
				}
				for _, card := range private {
					checked++
					if strings.Contains(string(wire), `"`+card.String()+`"`) {
						t.Fatalf("seq %d: %s's update carries %s's card %s:\n%s", r.seq, other, owner, card, wire)
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no hole cards were checked")
	}
}
