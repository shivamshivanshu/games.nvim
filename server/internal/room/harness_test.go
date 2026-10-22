package room

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games/chess"
	"github.com/shivamshivanshu/games.nvim/server/internal/clocktest"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/prototest"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
)

const waitTimeout = 5 * time.Second

type fakePersister struct {
	mu       sync.Mutex
	steps    []store.StepRecord
	finished *store.Finish
	fail     error
	attempts int
}

func (p *fakePersister) AppendSteps(_ context.Context, _ game.GameID, steps ...store.StepRecord) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.attempts++
	if p.fail != nil {
		return p.fail
	}
	p.steps = append(p.steps, steps...)
	return nil
}

func (p *fakePersister) FinishGame(_ context.Context, _ game.GameID, f store.Finish) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.attempts++
	if p.fail != nil {
		return p.fail
	}
	p.finished = &f
	return nil
}

func (p *fakePersister) setFail(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fail = err
}

func (p *fakePersister) attempted() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.attempts
}

func (p *fakePersister) recorded() ([]store.StepRecord, *store.Finish) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.steps), p.finished
}

type recordingBroadcaster struct {
	mu      sync.Mutex
	updates map[game.UserID][]protocol.GameUpdate
}

func (b *recordingBroadcaster) SendUpdate(to game.UserID, u protocol.GameUpdate) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.updates == nil {
		b.updates = map[game.UserID][]protocol.GameUpdate{}
	}
	b.updates[to] = append(b.updates[to], u)
}

func (b *recordingBroadcaster) last(user game.UserID) (protocol.GameUpdate, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	got := b.updates[user]
	if len(got) == 0 {
		return protocol.GameUpdate{}, 0
	}
	return got[len(got)-1], len(got)
}

type harness struct {
	room    *Room
	params  Params
	persist *fakePersister
	sent    *recordingBroadcaster
	clock   *clocktest.FakeClock
	cancel  context.CancelFunc
}

func newHarness(t *testing.T, timeControl game.TimeControl) *harness {
	t.Helper()
	return startHarness(t, game.Adapt(chess.Rules{}), `{"color":"white","time_control":"`+string(timeControl)+`"}`, prototest.Bob)
}

func startHarness(t *testing.T, g game.Game, options string, invitees ...game.UserID) *harness {
	t.Helper()
	opts, err := g.ResolveOptions(json.RawMessage(options))
	if err != nil {
		t.Fatal(err)
	}
	const seed = 42
	state, seats, err := g.New(opts, prototest.Alice, invitees, game.StepRNG(seed, 0))
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{
		persist: &fakePersister{},
		sent:    &recordingBroadcaster{},
		clock:   clocktest.New(clocktest.Epoch),
	}
	players, err := game.PlayersOf(game.Participants(prototest.Alice, invitees), seats)
	if err != nil {
		t.Fatal(err)
	}
	h.params = Params{
		ID: "g1", Game: g, Options: opts, Seed: seed, Players: players,
		Persist: h.persist, Broadcast: h.sent, Time: h.clock,
	}
	ctx, cancel := context.WithCancel(t.Context())
	h.cancel = cancel
	t.Cleanup(cancel)
	h.room = Start(ctx, newSetup(t, h.params), state)
	return h
}

func newSetup(t *testing.T, p Params) Setup {
	t.Helper()
	g, err := NewSetup(p)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func move(san string) json.RawMessage {
	return json.RawMessage(`{"kind":"move","move":"` + san + `"}`)
}

func (h *harness) play(t *testing.T, user game.UserID, seq game.Seq, san string) game.Seq {
	t.Helper()
	next, err := h.room.Act(t.Context(), user, seq, move(san))
	if err != nil {
		t.Fatalf("%s plays %s at seq %d: %v", user, san, seq, err)
	}
	return next
}

func (h *harness) control(t *testing.T, user game.UserID, seq game.Seq, c protocol.Control) game.Seq {
	t.Helper()
	next, err := h.room.Control(t.Context(), user, seq, c)
	if err != nil {
		t.Fatalf("%s %s at seq %d: %v", user, c, seq, err)
	}
	return next
}

func (h *harness) snapshot(t *testing.T, user game.UserID) protocol.GameUpdate {
	t.Helper()
	return h.fullSnapshot(t, user).Update
}

func (h *harness) fullSnapshot(t *testing.T, user game.UserID) Snapshot {
	t.Helper()
	s, err := h.room.Snapshot(t.Context(), user)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func waitDone(t *testing.T, r *Room) {
	t.Helper()
	select {
	case <-r.Done():
	case <-time.After(waitTimeout):
		t.Fatal("room did not finish")
	}
}
