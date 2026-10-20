package room

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/clock"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

var (
	ErrStale      = protocol.Error{Code: protocol.CodeStale, Msg: "room: stale seq"}
	ErrNotAPlayer = protocol.Error{Code: protocol.CodeNotFound, Msg: "room: not a player in this game"}
	errGameOver   = protocol.Error{Code: protocol.CodeIllegalAction, Msg: "game is over"}
)

type Persister interface {
	AppendStep(ctx context.Context, id game.GameID, step store.StepRecord) error
	FinishGame(ctx context.Context, id game.GameID, step store.StepRecord, reason game.ResultReason, outcomes map[game.UserID]store.Outcome) error
}

type Broadcaster interface {
	SendUpdate(to game.UserID, u protocol.GameUpdate)
}

type Params struct {
	ID         game.GameID
	Game       game.Game
	Options    game.Options
	Seed       game.Seed
	Challenger game.UserID
	Players    []game.Player
	CreatedAt  time.Time
	Persist    Persister
	Broadcast  Broadcaster
	Time       clock.Clock
}

// Setup is a validated game; once built, Start cannot fail.
type Setup struct {
	Params
	info  game.Info
	clock game.ClockConfig
	seats map[game.UserID]game.Seat
	users map[game.Seat]game.UserID
}

func NewSetup(p Params) (Setup, error) {
	if len(p.Players) == 0 {
		return Setup{}, fmt.Errorf("room: %s: no players", p.ID)
	}
	if !slices.ContainsFunc(p.Players, func(player game.Player) bool { return player.User == p.Challenger }) {
		return Setup{}, fmt.Errorf("room: %s: challenger %s is not a player", p.ID, p.Challenger)
	}
	clock, err := p.Options.Clock()
	if err != nil {
		return Setup{}, fmt.Errorf("room: %s: %w", p.ID, err)
	}
	g := Setup{
		Params: p,
		info:   p.Game.Info(),
		clock:  clock,
		seats:  make(map[game.UserID]game.Seat, len(p.Players)),
		users:  make(map[game.Seat]game.UserID, len(p.Players)),
	}
	for _, player := range p.Players {
		g.seats[player.User] = player.Seat
		g.users[player.Seat] = player.User
	}
	return g, nil
}

func (p Params) invitees() []game.UserID {
	users := xslices.Map(p.Players, func(player game.Player) game.UserID { return player.User })
	return slices.DeleteFunc(users, func(u game.UserID) bool { return u == p.Challenger })
}

func (g Setup) usersAt(seats []game.Seat) []game.UserID {
	return xslices.Map(seats, func(seat game.Seat) game.UserID { return g.users[seat] })
}

// roomState is the per-step state; apply returns a new value so a rejected or
// unpersisted step leaves the current one untouched.
type roomState struct {
	game       game.ErasedState
	seq        game.Seq
	acted      map[game.UserID]struct{}
	drawAgreed []game.UserID
	imposed    *game.Result
	events     []json.RawMessage
	remaining  map[game.UserID]time.Duration
	since      time.Time
	lastStepAt time.Time
}

func initialState(g Setup, state game.ErasedState, now time.Time) roomState {
	s := roomState{game: state, events: []json.RawMessage{}, since: now, lastStepAt: g.CreatedAt}
	if g.clock.Enabled {
		s.remaining = make(map[game.UserID]time.Duration, len(g.Players))
		for _, player := range g.Players {
			s.remaining[player.User] = g.clock.Initial
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

func (s roomState) toAct(g Setup) []game.UserID {
	if s.finished() {
		return []game.UserID{}
	}
	return g.usersAt(s.game.ToAct())
}

func (s roomState) apply(g Setup, st step) (roomState, error) {
	next := s
	next.seq = st.seq
	next.remaining = s.charged(g, st.at)
	next.since, next.lastStepAt = st.at, st.at
	next.events = []json.RawMessage{}
	var err error
	switch st.kind {
	case store.StepAction:
		next, err = next.applyAction(g, st)
	case store.StepControl:
		next, err = next.applyControl(g, st)
	case store.StepTimeout:
		next, err = next.applyTimeout(g, st)
	default:
		err = fmt.Errorf("room: unknown step kind %q", st.kind)
	}
	if err != nil {
		return roomState{}, err
	}
	return next, nil
}

func (s roomState) applyAction(g Setup, st step) (roomState, error) {
	if s.finished() {
		return s, errGameOver
	}
	state, events, err := s.game.Apply(g.seats[st.user], st.action, game.StepRNG(g.Seed, st.seq))
	if err != nil {
		return s, err
	}
	if s.clockRunning(g) {
		s.remaining[st.user] += g.clock.Increment
	}
	s.game, s.events = state, events
	s.drawAgreed = nil
	s.acted = xslices.With(s.acted, st.user, struct{}{})
	return s, nil
}

func (s roomState) applyTimeout(g Setup, st step) (roomState, error) {
	if s.finished() {
		return s, errGameOver
	}
	s, err := s.eliminate(g, st.user, game.ReasonTimeout)
	if err != nil {
		return s, err
	}
	if s.remaining != nil {
		s.remaining[st.user] = 0
	}
	return s, nil
}

func (s roomState) eliminate(g Setup, user game.UserID, reason game.ResultReason) (roomState, error) {
	state, events, err := s.game.Eliminate(g.seats[user], reason)
	if err != nil {
		return s, err
	}
	s.game, s.events = state, events
	return s, nil
}

func (s roomState) outcomes(g Setup) (game.ResultReason, map[game.UserID]store.Outcome) {
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

func (s roomState) update(g Setup, user game.UserID, at time.Time) (protocol.GameUpdate, error) {
	view, err := s.game.View(g.seats[user])
	if err != nil {
		return protocol.GameUpdate{}, fmt.Errorf("room: view for %s: %w", user, err)
	}
	var wireResult *protocol.WireResult
	if result, done := s.result(); done {
		kind, err := protocol.ResultKindOf(result.Kind)
		if err != nil {
			return protocol.GameUpdate{}, err
		}
		wireResult = &protocol.WireResult{Kind: kind, Winners: g.usersAt(result.Winners), Reason: result.Reason}
	}
	return protocol.GameUpdate{
		GameID:     g.ID,
		Game:       g.info.Type,
		Options:    maps.Clone(g.Options),
		Seq:        s.seq,
		Players:    slices.Clone(g.Players),
		ToAct:      s.toAct(g),
		Result:     wireResult,
		Clock:      s.wireClock(g, at),
		DrawAgreed: slices.Clone(s.drawAgreed),
		Controls:   s.controls(g, user),
		Events:     slices.Clone(s.events),
		View:       view,
	}, nil
}

type Room struct {
	setup  Setup
	ctx    context.Context
	cur    roomState
	timer  clock.Timer
	inbox  chan func()
	done   chan struct{}
	exited chan struct{}
}

func Start(ctx context.Context, g Setup, state game.ErasedState) *Room {
	return launch(ctx, g, initialState(g, state, g.Time.Now()))
}

func launch(ctx context.Context, g Setup, s roomState) *Room {
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

func (r *Room) Act(ctx context.Context, user game.UserID, seq game.Seq, action json.RawMessage) (game.Seq, error) {
	return r.submit(ctx, seq, step{user: user, kind: store.StepAction, action: action})
}

func (r *Room) Control(ctx context.Context, user game.UserID, seq game.Seq, c protocol.Control) (game.Seq, error) {
	return r.submit(ctx, seq, step{user: user, kind: store.StepControl, control: c})
}

type Snapshot struct {
	Update     protocol.GameUpdate
	LastStepAt time.Time
}

func (r *Room) Snapshot(ctx context.Context, user game.UserID) (Snapshot, error) {
	snapshot := func() (Snapshot, error) {
		if _, ok := r.setup.seats[user]; !ok {
			return Snapshot{}, ErrNotAPlayer
		}
		u, err := r.cur.update(r.setup, user, r.setup.Time.Now())
		return Snapshot{Update: u, LastStepAt: r.cur.lastStepAt}, err
	}
	// Once the actor has exited, its state is frozen and safe to read from here.
	return ask(ctx, r, snapshot, snapshot)
}

func (r *Room) rejectAfterExit() (game.Seq, error) {
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

func (r *Room) submit(ctx context.Context, seq game.Seq, st step) (game.Seq, error) {
	return ask(ctx, r, func() (game.Seq, error) {
		if _, ok := r.setup.seats[st.user]; !ok {
			return 0, ErrNotAPlayer
		}
		if seq != r.cur.seq {
			return 0, ErrStale
		}
		st.seq, st.at = seq+1, r.setup.Time.Now()
		return r.commit(st)
	}, r.rejectAfterExit)
}

func (r *Room) commit(st step) (game.Seq, error) {
	next, err := r.cur.apply(r.setup, st)
	if err != nil {
		return 0, err
	}
	rec, err := st.record(next.remaining)
	if err != nil {
		return 0, err
	}
	if err := r.persist(rec, next); err != nil {
		return 0, err
	}
	r.cur = next
	r.broadcast()
	r.armTimer()
	return next.seq, nil
}

// persist runs on the room's context, not the request's: a client hanging up
// mid-write must not leave memory behind a row that already committed.
func (r *Room) persist(rec store.StepRecord, next roomState) error {
	if !next.finished() {
		return r.setup.Persist.AppendStep(r.ctx, r.setup.ID, rec)
	}
	reason, outcomes := next.outcomes(r.setup)
	return r.setup.Persist.FinishGame(r.ctx, r.setup.ID, rec, reason, outcomes)
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
