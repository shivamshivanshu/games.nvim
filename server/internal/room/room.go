package room

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"games.nvim/server/game"
	"games.nvim/server/internal/protocol"
	"games.nvim/server/internal/store"
)

var (
	ErrStale      = errors.New("room: stale seq")
	ErrNotAPlayer = errors.New("room: not a player in this game")
)

var errGameOver = game.IllegalActionError{Msg: "game is over"}

type Persister interface {
	AppendStep(ctx context.Context, id game.GameID, step store.StepRecord) error
	FinishGame(ctx context.Context, id game.GameID, step store.StepRecord, reason game.ResultReason, outcomes map[game.UserID]store.Outcome) error
}

type Broadcaster interface {
	SendUpdate(to game.UserID, u protocol.GameUpdate)
}

type Timer interface{ Stop() bool }

type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

type Params struct {
	ID        game.GameID
	Game      game.Game
	Options   game.Options
	Seed      uint64
	Players   []store.PlayerRecord
	Clock     game.ClockConfig
	Persist   Persister
	Broadcast Broadcaster
	Time      Clock
}

// setup is everything about a game that is fixed when it starts.
type setup struct {
	Params
	gameType game.GameType
	caps     []game.Capability
	seats    map[game.UserID]game.Seat
	users    map[game.Seat]game.UserID
}

func newSetup(p Params) setup {
	info := p.Game.Info()
	g := setup{
		Params:   p,
		gameType: info.Type,
		caps:     info.Capabilities,
		seats:    make(map[game.UserID]game.Seat, len(p.Players)),
		users:    make(map[game.Seat]game.UserID, len(p.Players)),
	}
	for _, player := range p.Players {
		g.seats[player.User] = player.Seat
		g.users[player.Seat] = player.User
	}
	return g
}

func (g setup) usersAt(seats []game.Seat) []game.UserID {
	users := make([]game.UserID, 0, len(seats))
	for _, seat := range seats {
		users = append(users, g.users[seat])
	}
	return users
}

// roomState is the per-step state; apply returns a new value so a rejected or
// unpersisted step leaves the current one untouched.
type roomState struct {
	game       game.ErasedState
	seq        uint64
	acted      map[game.UserID]bool
	drawAgreed []game.UserID
	imposed    *game.Result
	events     []json.RawMessage
	remaining  map[game.UserID]time.Duration
	since      time.Time
}

func initialState(g setup, state game.ErasedState, now time.Time) roomState {
	s := roomState{game: state, events: []json.RawMessage{}, since: now}
	if g.Clock.Enabled {
		s.remaining = make(map[game.UserID]time.Duration, len(g.Players))
		for _, player := range g.Players {
			s.remaining[player.User] = g.Clock.Initial
		}
	}
	return s
}

func (s roomState) result() (game.Result, bool) {
	if s.imposed != nil {
		return *s.imposed, true
	}
	return s.game.Outcome()
}

func (s roomState) finished() bool {
	_, done := s.result()
	return done
}

func (s roomState) toAct(g setup) []game.UserID {
	if s.finished() {
		return []game.UserID{}
	}
	return g.usersAt(s.game.ToAct())
}

func (s roomState) apply(g setup, rec store.StepRecord) (roomState, error) {
	next := s
	next.seq = rec.Seq
	next.remaining = s.charged(g, rec.At)
	next.since = rec.At
	var events []game.Event
	var err error
	switch rec.Kind {
	case store.StepAction:
		next, events, err = next.applyAction(g, rec, s.clockRunning(g))
	case store.StepControl:
		next, events, err = next.applyControl(g, rec)
	case store.StepTimeout:
		next, events, err = next.applyTimeout(g, rec)
	default:
		err = fmt.Errorf("room: unknown step kind %q", rec.Kind)
	}
	if err != nil {
		return roomState{}, err
	}
	if next.events, err = encodeEvents(events); err != nil {
		return roomState{}, err
	}
	return next, nil
}

func (s roomState) applyAction(g setup, rec store.StepRecord, clockWasRunning bool) (roomState, []game.Event, error) {
	if s.finished() {
		return s, nil, errGameOver
	}
	state, events, err := s.game.Apply(g.seats[rec.User], rec.Payload, game.StepRNG(g.Seed, rec.Seq))
	if err != nil {
		return s, nil, err
	}
	s.game = state
	s.drawAgreed = nil
	s.acted = withEntry(s.acted, rec.User, true)
	if clockWasRunning {
		s.remaining[rec.User] += g.Clock.Increment
	}
	return s, events, nil
}

func (s roomState) applyTimeout(g setup, rec store.StepRecord) (roomState, []game.Event, error) {
	if s.finished() {
		return s, nil, errGameOver
	}
	state, events := s.game.Eliminate(g.seats[rec.User], game.ReasonTimeout)
	s.game = state
	if s.remaining != nil {
		s.remaining[rec.User] = 0
	}
	return s, events, nil
}

func withEntry[K comparable, V any](m map[K]V, k K, v V) map[K]V {
	out := maps.Clone(m)
	if out == nil {
		out = map[K]V{}
	}
	out[k] = v
	return out
}

func encodeEvents(events []game.Event) ([]json.RawMessage, error) {
	encoded := make([]json.RawMessage, 0, len(events))
	for _, e := range events {
		raw, err := protocol.EncodeEvent(e)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, raw)
	}
	return encoded, nil
}

func (s roomState) outcomes(g setup) (game.ResultReason, map[game.UserID]store.Outcome) {
	result, _ := s.result()
	outcomes := make(map[game.UserID]store.Outcome, len(g.Players))
	for _, player := range g.Players {
		switch result.Kind {
		case game.ResultDecisive:
			outcomes[player.User] = store.OutcomeLoss
			if slices.Contains(result.Winners, player.Seat) {
				outcomes[player.User] = store.OutcomeWin
			}
		case game.ResultDraw:
			outcomes[player.User] = store.OutcomeDraw
		}
	}
	return result.Reason, outcomes
}

func (s roomState) update(g setup, user game.UserID, at time.Time) (protocol.GameUpdate, error) {
	view, err := s.game.View(g.seats[user])
	if err != nil {
		return protocol.GameUpdate{}, fmt.Errorf("room: view for %s: %w", user, err)
	}
	options, err := json.Marshal(g.Options)
	if err != nil {
		return protocol.GameUpdate{}, fmt.Errorf("room: encode options: %w", err)
	}
	players := make([]protocol.PlayerSeat, 0, len(g.Players))
	for _, player := range g.Players {
		players = append(players, protocol.PlayerSeat{User: player.User, Seat: player.Seat})
	}
	var wireResult *protocol.WireResult
	if result, done := s.result(); done {
		wireResult = &protocol.WireResult{Winners: g.usersAt(result.Winners), Reason: result.Reason}
	}
	return protocol.GameUpdate{
		GameID:     g.ID,
		Game:       g.gameType,
		Options:    options,
		Seq:        s.seq,
		Players:    players,
		ToAct:      s.toAct(g),
		Result:     wireResult,
		Clock:      s.wireClock(g, at),
		DrawAgreed: slices.Clone(s.drawAgreed),
		Controls:   s.controls(g, user),
		Events:     s.events,
		View:       view,
	}, nil
}

type Room struct {
	setup  setup
	ctx    context.Context
	cur    roomState
	timer  Timer
	inbox  chan func()
	done   chan struct{}
	exited chan struct{}
}

func New(ctx context.Context, p Params, state game.ErasedState) *Room {
	g := newSetup(p)
	return start(ctx, g, initialState(g, state, p.Time.Now()))
}

func start(ctx context.Context, g setup, s roomState) *Room {
	r := &Room{
		setup:  g,
		ctx:    ctx,
		cur:    s,
		inbox:  make(chan func()),
		done:   make(chan struct{}),
		exited: make(chan struct{}),
	}
	go r.run()
	return r
}

func (r *Room) Done() <-chan struct{} { return r.done }

func (r *Room) run() {
	defer close(r.exited)
	defer r.stopTimer()
	r.armTimer()
	for !r.cur.finished() {
		select {
		case msg := <-r.inbox:
			r.expireFlagged()
			msg()
		case <-r.ctx.Done():
			return
		}
	}
	close(r.done)
}

func (r *Room) Act(ctx context.Context, user game.UserID, seq uint64, action json.RawMessage) (uint64, error) {
	return ask(ctx, r, func() (uint64, error) {
		return r.submit(ctx, user, seq, store.StepAction, action)
	}, r.rejectAfterExit)
}

func (r *Room) Control(ctx context.Context, user game.UserID, seq uint64, c protocol.Control) (uint64, error) {
	payload, err := json.Marshal(c)
	if err != nil {
		return 0, err
	}
	return ask(ctx, r, func() (uint64, error) {
		return r.submit(ctx, user, seq, store.StepControl, payload)
	}, r.rejectAfterExit)
}

func (r *Room) Snapshot(user game.UserID) (protocol.GameUpdate, error) {
	snapshot := func() (protocol.GameUpdate, error) {
		if _, ok := r.setup.seats[user]; !ok {
			return protocol.GameUpdate{}, ErrNotAPlayer
		}
		return r.cur.update(r.setup, user, r.setup.Time.Now())
	}
	// Once the actor has exited, its state is frozen and safe to read from here.
	return ask(context.Background(), r, snapshot, snapshot)
}

func (r *Room) rejectAfterExit() (uint64, error) {
	if r.cur.finished() {
		return 0, errGameOver
	}
	return 0, r.ctx.Err()
}

type reply[T any] struct {
	value T
	err   error
}

func ask[T any](ctx context.Context, r *Room, inActor, afterExit func() (T, error)) (T, error) {
	replies := make(chan reply[T], 1)
	msg := func() {
		value, err := inActor()
		replies <- reply[T]{value, err}
	}
	select {
	case r.inbox <- msg:
		got := <-replies
		return got.value, got.err
	case <-r.exited:
		return afterExit()
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

func (r *Room) submit(ctx context.Context, user game.UserID, seq uint64, kind store.StepKind, payload json.RawMessage) (uint64, error) {
	if _, ok := r.setup.seats[user]; !ok {
		return 0, ErrNotAPlayer
	}
	if seq != r.cur.seq {
		return 0, ErrStale
	}
	return r.commit(ctx, store.StepRecord{Seq: seq + 1, User: user, Kind: kind, Payload: payload, At: r.setup.Time.Now()})
}

func (r *Room) commit(ctx context.Context, rec store.StepRecord) (uint64, error) {
	next, err := r.cur.apply(r.setup, rec)
	if err != nil {
		return 0, err
	}
	rec.ClockRemainingMS = next.remainingMS()
	if err := r.persist(ctx, rec, next); err != nil {
		return 0, err
	}
	r.cur = next
	r.broadcast()
	r.armTimer()
	return next.seq, nil
}

func (r *Room) persist(ctx context.Context, rec store.StepRecord, next roomState) error {
	if !next.finished() {
		return r.setup.Persist.AppendStep(ctx, r.setup.ID, rec)
	}
	reason, outcomes := next.outcomes(r.setup)
	return r.setup.Persist.FinishGame(ctx, r.setup.ID, rec, reason, outcomes)
}

func (r *Room) broadcast() {
	for _, player := range r.setup.Players {
		u, err := r.cur.update(r.setup, player.User, r.cur.since)
		if err != nil {
			slog.Error("room: build update", "game", r.setup.ID, "user", player.User, "err", err)
			continue
		}
		r.setup.Broadcast.SendUpdate(player.User, u)
	}
}
