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
	"github.com/shivamshivanshu/games.nvim/server/internal/archive"
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
	FinishGame(ctx context.Context, id game.GameID, f store.Finish) error
}

// Broadcaster gets one update per player for each step. Those updates share
// their slices and maps, so SendUpdate must not modify u.
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
	return slices.DeleteFunc(game.Users(p.Players), func(u game.UserID) bool { return u == p.Challenger })
}

func (g Setup) usersAt(seats []game.Seat) []game.UserID {
	return xslices.Map(seats, func(seat game.Seat) game.UserID { return g.users[seat] })
}

func (g Setup) finished(s roomState, at time.Time) (archive.Finished, error) {
	ending, err := s.ending(g)
	if err != nil {
		return archive.Finished{}, err
	}
	views, err := s.views(g)
	if err != nil {
		return archive.Finished{}, err
	}
	return archive.Finished{
		ID: g.ID, Type: g.info.Type, Options: g.Options, Players: g.Players, StartedAt: g.CreatedAt,
		Finish: store.Finish{At: at, Seq: s.seq, Ending: ending, Views: views, Exports: s.exports(g)},
	}.Sealed()
}

// roomState is the per-step state; apply returns a new value so a rejected or
// unpersisted step leaves the current one untouched.
type roomState struct {
	game       game.ErasedState
	seq        game.Seq
	acted      map[game.UserID]struct{}
	eliminated map[game.UserID]struct{}
	drawAgreed []game.UserID
	imposed    *game.Result
	events     []json.RawMessage
	remaining  map[game.UserID]time.Duration
	since      time.Time
	lastStepAt time.Time
	// freshFrom is the oldest seq a player may act on: the steps since then
	// did not make it stale (see makesStale).
	freshFrom game.Seq
	// nextTickAt is when a ticking game's next tick is due, zero when none is.
	nextTickAt time.Time
	final      *archive.Finished
	// result, toAct and clockRunning follow from the fields above; settled
	// recomputes them whenever those change.
	result       *game.Result
	toAct        []game.UserID
	clockRunning bool
}

func initialState(g Setup, state game.ErasedState, now time.Time) roomState {
	s := roomState{game: state, events: state.OpeningEvents(), since: now, lastStepAt: g.CreatedAt, nextTickAt: tickDue(state, now)}
	if g.clock.Enabled {
		s.remaining = make(map[game.UserID]time.Duration, len(g.Players))
		for _, player := range g.Players {
			s.remaining[player.User] = g.clock.Initial
		}
	}
	return s.settled(g)
}

func (s roomState) settled(g Setup) roomState {
	s.result = s.imposed
	if s.result == nil {
		if result, done := s.game.Outcome(); done {
			s.result = &result
		}
	}
	s.toAct = []game.UserID{}
	if s.result == nil {
		s.toAct = g.usersAt(s.game.ToAct())
	}
	s.clockRunning = g.clock.Enabled && s.result == nil && s.allActiveActed(g)
	return s
}

func (s roomState) impose(g Setup, result game.Result) roomState {
	s.imposed = &result
	s.game = s.game.Conclude(result)
	return s.settled(g)
}

func (s roomState) finished() bool { return s.result != nil }

func (s roomState) hasTurn(user game.UserID) bool { return slices.Contains(s.toAct, user) }

func (s roomState) apply(g Setup, st step) (roomState, error) {
	next := s
	next.events = []json.RawMessage{}
	var err error
	switch st.kind {
	case store.StepAction:
		next, err = next.applyAction(g, st)
	case store.StepControl:
		next, err = next.applyControl(g, st)
	case store.StepTimeout:
		next, err = next.applyTimeout(g, st)
	case store.StepTick:
		next, err = next.applyTick(g, st)
	case store.StepPause:
		next = next.applyPause(g)
	default:
		err = fmt.Errorf("room: unknown step kind %q", st.kind)
	}
	if err != nil {
		return roomState{}, err
	}
	next.seq = st.seq
	next.since, next.lastStepAt = st.at, st.at
	next.remaining = s.clockAfter(g, st, next)
	next.nextTickAt = s.tickAfter(st, next)
	if s.makesStale(st) {
		next.freshFrom = st.seq
	}
	return next, nil
}

// makesStale is whether st makes the seqs before it stale. Ticks and pauses
// never do, nor does an action in a real-time game, paused or not: its input
// is an intent that holds against any state, so a stale check would only lose
// the player's newest key.
func (s roomState) makesStale(st step) bool {
	if st.paces() {
		return false
	}
	return st.kind != store.StepAction || !s.game.RealTime()
}

func (s roomState) ticking() bool { return !s.nextTickAt.IsZero() }

func (s roomState) applyAction(g Setup, st step) (roomState, error) {
	if s.finished() {
		return s, errGameOver
	}
	state, events, err := s.game.Apply(g.seats[st.user], st.action, game.StepRNG(g.Seed, st.seq))
	if err != nil {
		return s, err
	}
	next := s
	next.game, next.events = state, events
	next.drawAgreed = nil
	next.acted = xslices.With(s.acted, st.user, struct{}{})
	return next.withKnockouts(g).settled(g), nil
}

func (s roomState) applyTimeout(g Setup, st step) (roomState, error) {
	if s.finished() {
		return s, errGameOver
	}
	return s.eliminate(g, st.user, game.ReasonTimeout)
}

func (s roomState) eliminate(g Setup, user game.UserID, reason game.ResultReason) (roomState, error) {
	state, events, err := s.game.Eliminate(g.seats[user], reason)
	if err != nil {
		return s, err
	}
	s.game, s.events = state, events
	s.eliminated = xslices.With(s.eliminated, user, struct{}{})
	s.drawAgreed = slices.DeleteFunc(slices.Clone(s.drawAgreed), func(u game.UserID) bool { return u == user })
	return s.withKnockouts(g).settled(g).imposeAgreedDraw(g), nil
}

// withKnockouts counts the seats the rules put out as eliminated, so they
// lose their controls as if they had resigned.
func (s roomState) withKnockouts(g Setup) roomState {
	for _, user := range g.usersAt(s.game.KnockedOut()) {
		s.eliminated = xslices.With(s.eliminated, user, struct{}{})
	}
	return s
}

func (s roomState) isEliminated(user game.UserID) bool {
	_, out := s.eliminated[user]
	return out
}

func (s roomState) everyActive(g Setup, holds func(game.UserID) bool) bool {
	return !slices.ContainsFunc(g.Players, func(p game.Player) bool { return !s.isEliminated(p.User) && !holds(p.User) })
}

func (s roomState) allActiveActed(g Setup) bool {
	return s.everyActive(g, func(u game.UserID) bool {
		_, acted := s.acted[u]
		return acted
	})
}

func (s roomState) ending(g Setup) (store.Ending, error) {
	result := *s.result
	scores, scored := s.gameScores()
	standings := make(map[game.UserID]store.Standing, len(g.Players))
	for _, player := range g.Players {
		standing, err := s.standing(result, player)
		if err != nil {
			return store.Ending{}, err
		}
		if scored {
			standing.Score = scores[player.Seat]
		}
		standings[player.User] = standing
	}
	return store.Ending{Kind: result.Kind, Reason: result.Reason, Standings: standings}, nil
}

// standing keeps an eliminated player's loss when the core imposes the ending
// (draw agreed, abort), since that result is decided only by those remaining.
func (s roomState) standing(result game.Result, player game.Player) (store.Standing, error) {
	if s.imposed != nil && s.isEliminated(player.User) {
		return store.Standing{Outcome: store.OutcomeLoss, Score: -1}, nil
	}
	return standingOf(result, player.Seat)
}

// standingOf scores winners +1 and losers -1, and a draw or an abort 0.
func standingOf(result game.Result, seat game.Seat) (store.Standing, error) {
	switch result.Kind {
	case game.ResultDecisive:
		if slices.Contains(result.Winners, seat) {
			return store.Standing{Outcome: store.OutcomeWin, Score: 1}, nil
		}
		return store.Standing{Outcome: store.OutcomeLoss, Score: -1}, nil
	case game.ResultDraw:
		return store.Standing{Outcome: store.OutcomeDraw}, nil
	case game.ResultAborted:
		return store.Standing{Outcome: store.OutcomeNone}, nil
	}
	return store.Standing{}, fmt.Errorf("room: unknown result kind %d", result.Kind)
}

// gameScores are the game's own when its final state is a game.Scorer and the
// result is the game's, not one the core imposed (draw agreed, abort).
func (s roomState) gameScores() (map[game.Seat]int64, bool) {
	if s.imposed != nil {
		return nil, false
	}
	return s.game.Scores()
}

func (s roomState) view(g Setup, user game.UserID) (json.RawMessage, error) {
	view, err := s.game.View(g.seats[user])
	if err != nil {
		return nil, fmt.Errorf("room: view for %s: %w", user, err)
	}
	return view, nil
}

func (s roomState) views(g Setup) (map[game.UserID]json.RawMessage, error) {
	views := make(map[game.UserID]json.RawMessage, len(g.Players))
	for _, player := range g.Players {
		view, err := s.view(g, player.User)
		if err != nil {
			return nil, err
		}
		views[player.User] = view
	}
	return views, nil
}

func (s roomState) exports(g Setup) map[string]string {
	files := s.game.Exports(game.ExportContext{Players: slices.Clone(g.Players), StartedAt: g.CreatedAt, Result: *s.result})
	return xslices.MapValues(files, func(content []byte) string { return string(content) })
}

// update is what user sees at at: a finished game as it ended, or the live game.
func (s roomState) update(g Setup, user game.UserID, at time.Time) (protocol.GameUpdate, error) {
	if s.final != nil {
		return s.final.ViewedBy(user), nil
	}
	return s.addressed(g, s.live(g, at), user)
}

// live is the ongoing game at at as every player sees it.
func (s roomState) live(g Setup, at time.Time) protocol.GameUpdate {
	return protocol.GameUpdate{
		GameID:     g.ID,
		Game:       g.info.Type,
		Options:    maps.Clone(g.Options),
		Seq:        s.seq,
		Players:    slices.Clone(g.Players),
		ToAct:      slices.Clone(s.toAct),
		Clock:      s.wireClock(g, at),
		DrawAgreed: append([]game.UserID{}, s.drawAgreed...),
		Events:     slices.Clone(s.events),
		StartedAt:  protocol.UnixMSOf(g.CreatedAt),
	}
}

// stepUpdate is what the step that reached s broadcasts to every player; a
// finishing step adds its events and clock to the sealed game.
func (s roomState) stepUpdate(g Setup) protocol.GameUpdate {
	if s.final == nil {
		return s.live(g, s.since)
	}
	u := s.final.Update()
	u.Events, u.Clock = slices.Clone(s.events), s.wireClock(g, s.since)
	return u
}

// addressed is u with the view and controls of user alone.
func (s roomState) addressed(g Setup, u protocol.GameUpdate, user game.UserID) (protocol.GameUpdate, error) {
	view, err := s.view(g, user)
	if err != nil {
		return protocol.GameUpdate{}, err
	}
	u.View, u.Controls = view, s.controls(g, user)
	return u, nil
}

type Room struct {
	setup  Setup
	ctx    context.Context
	cur    roomState
	timer  clock.Timer
	inbox  chan func()
	done   chan struct{}
	exited chan struct{}

	// pauseWanted is a pause asked for, while its player is away, that has
	// yet to persist.
	pauseWanted func() bool
}

func Start(ctx context.Context, g Setup, state game.ErasedState) *Room {
	return launch(ctx, g, initialState(g, state, g.Time.Now()))
}

func launch(ctx context.Context, g Setup, s roomState) *Room {
	r := newRoom(ctx, g, s)
	go r.run()
	return r
}

func newRoom(ctx context.Context, g Setup, s roomState) *Room {
	return &Room{
		setup:  g,
		ctx:    ctx,
		cur:    s,
		inbox:  make(chan func()),
		done:   make(chan struct{}),
		exited: make(chan struct{}),
	}
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
			r.tickIfDue()
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
		if seq < r.cur.freshFrom || seq > r.cur.seq {
			return 0, ErrStale
		}
		st.seq, st.at = r.cur.seq+1, r.setup.Time.Now()
		return r.commit(st)
	}, r.rejectAfterExit)
}

func (r *Room) commit(st step) (game.Seq, error) {
	next, err := r.cur.apply(r.setup, st)
	if err != nil {
		return 0, err
	}
	if next, err = r.persist(st, next); err != nil {
		return 0, err
	}
	r.cur = next
	// Armed before the update goes out, so a test clock advanced on seeing
	// the update always finds the next wake.
	r.armTimer()
	r.broadcast()
	return next.seq, nil
}

// persist runs on the room's context, not the request's: a client hanging up
// mid-write must not leave memory behind a row that already committed. The
// finishing step is not stored: the finish replaces all steps with a summary.
func (r *Room) persist(st step, next roomState) (roomState, error) {
	if next.finished() {
		final, err := r.setup.finished(next, st.at)
		if err != nil {
			return roomState{}, err
		}
		next.final = &final
		return next, r.setup.Persist.FinishGame(r.ctx, r.setup.ID, final.Finish)
	}
	rec, err := st.record(next.remaining)
	if err != nil {
		return roomState{}, err
	}
	return next, r.setup.Persist.AppendStep(r.ctx, r.setup.ID, rec)
}

func (r *Room) broadcast() {
	shared := r.cur.stepUpdate(r.setup)
	for _, player := range r.setup.Players {
		u, err := r.cur.addressed(r.setup, shared, player.User)
		if err != nil {
			slog.Error("room: build update", "game", r.setup.ID, "user", player.User, "err", err)
			continue
		}
		r.setup.Broadcast.SendUpdate(player.User, u)
	}
}
