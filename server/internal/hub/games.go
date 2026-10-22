package hub

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/room"
	"github.com/shivamshivanshu/games.nvim/server/internal/store"
	"github.com/shivamshivanshu/games.nvim/server/internal/uuid"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

type liveGame struct {
	room    *room.Room
	players []game.UserID
}

func (h *Hub) startChallenge(c protocol.Challenge) (game.GameID, error) {
	id, _, err := h.startGame(c.Game, c.Options, c.From, c.To)
	if err != nil {
		slog.Error("hub: start game", "challenge", c.ChallengeID, "err", err)
	}
	return id, err
}

// startGame returns each player's first update; a solo game has no invitees.
func (h *Hub) startGame(typ game.GameType, opts game.Options, challenger game.UserID, invitees []game.UserID) (game.GameID, map[game.UserID]protocol.GameUpdate, error) {
	g, state, err := h.prepareGame(typ, opts, challenger, invitees)
	if err != nil {
		return "", nil, err
	}
	if err := h.persistNewGame(g); err != nil {
		return "", nil, err
	}
	return g.ID, h.launch(g, state), nil
}

func (h *Hub) prepareGame(typ game.GameType, opts game.Options, challenger game.UserID, invitees []game.UserID) (room.Setup, game.ErasedState, error) {
	g, err := h.gameType(typ)
	if err != nil {
		return room.Setup{}, nil, err
	}
	seed := h.deps.Seeds()
	state, seats, err := g.New(opts, challenger, invitees, game.StepRNG(seed, 0))
	if err != nil {
		return room.Setup{}, nil, err
	}
	players, err := game.PlayersOf(game.Participants(challenger, invitees), seats)
	if err != nil {
		return room.Setup{}, nil, fmt.Errorf("hub: %w", err)
	}
	setup, err := h.roomSetup(room.Params{
		ID:        game.GameID(uuid.NewV4()),
		Game:      g,
		Options:   opts,
		Seed:      seed,
		Players:   players,
		CreatedAt: h.deps.Time.Now(),
	})
	return setup, state, err
}

func (h *Hub) persistNewGame(p room.Setup) error {
	options, err := json.Marshal(p.Options)
	if err != nil {
		return err
	}
	return h.deps.Store.CreateGame(h.ctx, store.GameRecord{
		ID: p.ID, Type: p.Game.Info().Type, Options: options, Seed: p.Seed, Players: p.Players, CreatedAt: p.CreatedAt,
	})
}

// launch takes the first updates before tracking the room, so no request can
// reach the room ahead of them, and pushes exactly those.
func (h *Hub) launch(g room.Setup, state game.ErasedState) map[game.UserID]protocol.GameUpdate {
	r := room.Start(h.ctx, g, state)
	first := h.firstUpdates(r, g.Players)
	h.track(g.ID, r, g.Players)
	for user, u := range first {
		h.SendUpdate(user, u)
	}
	return first
}

func (h *Hub) firstUpdates(r *room.Room, players []game.Player) map[game.UserID]protocol.GameUpdate {
	first := make(map[game.UserID]protocol.GameUpdate, len(players))
	for _, p := range players {
		s, err := r.Snapshot(h.ctx, p.User)
		if err != nil {
			slog.Error("hub: initial update", "user", p.User, "err", err)
			continue
		}
		first[p.User] = s.Update
	}
	return first
}

func (h *Hub) restoreOngoing(ctx context.Context) error {
	recs, err := h.deps.Store.LoadOngoing(ctx)
	if err != nil {
		return fmt.Errorf("hub: load ongoing games: %w", err)
	}
	for _, rec := range recs {
		r, err := h.restore(h.ctx, rec)
		if err != nil {
			return err
		}
		h.track(rec.ID, r, rec.Players)
	}
	return nil
}

func (h *Hub) restore(ctx context.Context, rec store.GameRecord) (*room.Room, error) {
	g, opts, err := h.resolveGame(rec.Type, rec.Options)
	if err != nil {
		return nil, fmt.Errorf("hub: restore %s: %w", rec.ID, err)
	}
	setup, err := h.roomSetup(room.Params{
		ID:        rec.ID,
		Game:      g,
		Options:   opts,
		Seed:      rec.Seed,
		Players:   rec.Players,
		CreatedAt: rec.CreatedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("hub: restore %s: %w", rec.ID, err)
	}
	return room.Restore(ctx, setup, rec.Steps, h.away(game.Users(rec.Players)...))
}

func (h *Hub) roomSetup(p room.Params) (room.Setup, error) {
	p.Persist, p.Broadcast, p.Time = h.deps.Store, h, h.deps.Time
	return room.NewSetup(p)
}

func (h *Hub) track(id game.GameID, r *room.Room, players []game.Player) {
	h.mu.Lock()
	h.live[id] = &liveGame{room: r, players: game.Users(players)}
	h.mu.Unlock()
	go h.dropWhenFinished(id, r)
}

func (h *Hub) dropWhenFinished(id game.GameID, r *room.Room) {
	select {
	case <-r.Done():
		h.mu.Lock()
		delete(h.live, id)
		h.mu.Unlock()
	case <-h.ctx.Done():
	}
}

// Wait returns once every room has stopped, its last steps written, which
// they do once the hub's context is cancelled.
func (h *Hub) Wait() {
	h.mu.Lock()
	live := slices.Collect(maps.Values(h.live))
	h.mu.Unlock()
	for _, g := range live {
		<-g.room.Exited()
	}
}

func (h *Hub) liveRoom(id game.GameID) (*room.Room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	g, ok := h.live[id]
	if !ok {
		return nil, protocol.Error{Code: protocol.CodeNotFound, Msg: fmt.Sprintf("no ongoing game %s: %v", id, store.ErrNotFound)}
	}
	return g.room, nil
}

func (h *Hub) liveRoomsOf(user game.UserID) []*room.Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []*room.Room
	for _, g := range h.live {
		if slices.Contains(g.players, user) {
			out = append(out, g.room)
		}
	}
	return out
}

func (h *Hub) ongoingGames(ctx context.Context, user game.UserID) ([]protocol.GameUpdate, error) {
	var games []room.Snapshot
	for _, r := range h.liveRoomsOf(user) {
		s, err := r.Snapshot(ctx, user)
		if err != nil {
			return nil, err
		}
		if s.Update.Result == nil {
			games = append(games, s)
		}
	}
	slices.SortFunc(games, func(a, b room.Snapshot) int {
		return cmp.Or(
			cmp.Compare(turnRankOf(a.Update, user), turnRankOf(b.Update, user)),
			b.LastStepAt.Compare(a.LastStepAt),
			cmp.Compare(a.Update.GameID, b.Update.GameID),
		)
	})
	return xslices.Map(games, func(s room.Snapshot) protocol.GameUpdate { return s.Update }), nil
}

type turnRank int

const (
	yourTurn turnRank = iota
	notYourTurn
)

func turnRankOf(u protocol.GameUpdate, user game.UserID) turnRank {
	if slices.Contains(u.ToAct, user) {
		return yourTurn
	}
	return notYourTurn
}

func (h *Hub) finishedGames(ctx context.Context, user game.UserID) ([]protocol.GameUpdate, error) {
	recs, err := h.deps.Store.LoadFinished(ctx, user, h.deps.Limits.FinishedListLimit)
	if err != nil {
		return nil, err
	}
	return xslices.MapErr(recs, func(rec store.GameRecord) (protocol.GameUpdate, error) {
		return finishedUpdate(rec, user)
	})
}

// snapshot reads a finished game from its stored record; every ongoing game has a live room.
func (h *Hub) snapshot(ctx context.Context, id game.GameID, user game.UserID) (protocol.GameUpdate, error) {
	if r, err := h.liveRoom(id); err == nil {
		s, err := r.Snapshot(ctx, user)
		return s.Update, err
	}
	rec, err := h.deps.Store.LoadGame(ctx, id)
	if errors.Is(err, store.ErrNotFound) || err == nil && rec.Finish == nil {
		return protocol.GameUpdate{}, protocol.Error{Code: protocol.CodeNotFound, Msg: fmt.Sprintf("no game %s", id)}
	}
	if err != nil {
		return protocol.GameUpdate{}, err
	}
	return finishedUpdate(rec, user)
}

func (h *Hub) gameType(t game.GameType) (game.Game, error) {
	g, ok := h.gameTypes[t]
	if !ok {
		return nil, protocol.BadRequestf("unknown game %q", t)
	}
	return g, nil
}

// resolveGame is the game of type typ and the options raw gives it.
func (h *Hub) resolveGame(typ game.GameType, raw json.RawMessage) (game.Game, game.Options, error) {
	g, err := h.gameType(typ)
	if err != nil {
		return nil, nil, err
	}
	opts, err := g.ResolveOptions(raw)
	if err != nil {
		return nil, nil, err
	}
	return g, opts, nil
}

func newSeed() game.Seed {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return game.Seed(binary.LittleEndian.Uint64(b[:]))
}
