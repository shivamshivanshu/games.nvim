package hub

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"games.nvim/server/game"
	"games.nvim/server/internal/protocol"
	"games.nvim/server/internal/room"
	"games.nvim/server/internal/store"
	"games.nvim/server/internal/xslices"
)

type liveGame struct {
	room     *room.Room
	players  []game.UserID
	lastStep time.Time
}

func (h *Hub) startChallenge(c protocol.Challenge) (game.GameID, error) {
	id, err := h.startGame(c)
	if err != nil {
		slog.Error("hub: start game", "challenge", c.ChallengeID, "err", err)
	}
	return id, err
}

func (h *Hub) startGame(c protocol.Challenge) (game.GameID, error) {
	p, state, err := h.prepareGame(c)
	if err != nil {
		return "", err
	}
	now := h.deps.Time.Now()
	if err := h.persistNewGame(p, now); err != nil {
		return "", err
	}
	return p.ID, h.launch(p, state, now)
}

func (h *Hub) prepareGame(c protocol.Challenge) (room.Params, game.ErasedState, error) {
	g, err := h.gameType(c.Game)
	if err != nil {
		return room.Params{}, nil, err
	}
	seed := newSeed()
	state, seats, err := g.New(c.Options, c.From, c.To, game.StepRNG(seed, 0))
	if err != nil {
		return room.Params{}, nil, err
	}
	players, err := seated(c.Participants(), seats)
	if err != nil {
		return room.Params{}, nil, err
	}
	return h.roomParams(game.GameID(rand.Text()), g, c.Options, seed, c.From, players), state, nil
}

func (h *Hub) persistNewGame(p room.Params, createdAt time.Time) error {
	options, err := json.Marshal(p.Options)
	if err != nil {
		return err
	}
	return h.deps.Store.CreateGame(h.ctx, store.GameRecord{
		ID: p.ID, Type: p.Game.Info().Type, Options: options, Seed: p.Seed, Players: p.Players, CreatedAt: createdAt,
	})
}

func (h *Hub) launch(p room.Params, state game.ErasedState, createdAt time.Time) error {
	r, err := room.New(h.ctx, p, state)
	if err != nil {
		return err
	}
	h.track(p.ID, r, p.Players, createdAt)
	h.sendCurrent(r, p.Players)
	return nil
}

// seated keeps the challenger first, which store.GameRecord.Challenger relies on.
func seated(users []game.UserID, seats map[game.UserID]game.Seat) ([]game.Player, error) {
	if len(seats) != len(users) {
		return nil, fmt.Errorf("hub: game assigned %d seats for %d players", len(seats), len(users))
	}
	return xslices.MapErr(users, func(user game.UserID) (game.Player, error) {
		seat, ok := seats[user]
		if !ok {
			return game.Player{}, fmt.Errorf("hub: game assigned no seat to %s", user)
		}
		return game.Player{User: user, Seat: seat}, nil
	})
}

func (h *Hub) sendCurrent(r *room.Room, players []game.Player) {
	for _, p := range players {
		u, err := r.Snapshot(h.ctx, p.User)
		if err != nil {
			slog.Error("hub: initial update", "user", p.User, "err", err)
			continue
		}
		h.SendUpdate(p.User, u)
	}
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
		h.track(rec.ID, r, rec.Players, lastStepAt(rec))
	}
	return nil
}

func lastStepAt(rec store.GameRecord) time.Time {
	if len(rec.Steps) == 0 {
		return rec.CreatedAt
	}
	return rec.Steps[len(rec.Steps)-1].At
}

func (h *Hub) restore(ctx context.Context, rec store.GameRecord) (*room.Room, error) {
	g, err := h.gameType(rec.Type)
	if err != nil {
		return nil, fmt.Errorf("hub: restore %s: %w", rec.ID, err)
	}
	opts, err := g.ResolveOptions(rec.Options)
	if err != nil {
		return nil, fmt.Errorf("hub: restore %s: %w", rec.ID, err)
	}
	challenger, ok := rec.Challenger()
	if !ok {
		return nil, fmt.Errorf("hub: restore %s: no players", rec.ID)
	}
	return room.Restore(ctx, h.roomParams(rec.ID, g, opts, rec.Seed, challenger, rec.Players), rec.Steps)
}

func (h *Hub) roomParams(id game.GameID, g game.Game, opts game.Options, seed game.Seed, challenger game.UserID, players []game.Player) room.Params {
	return room.Params{
		ID:         id,
		Game:       g,
		Options:    opts,
		Seed:       seed,
		Challenger: challenger,
		Players:    players,
		Persist:    h.deps.Store,
		Broadcast:  h,
		Time:       h.deps.Time,
	}
}

func (h *Hub) track(id game.GameID, r *room.Room, players []game.Player, lastStep time.Time) {
	users := xslices.Map(players, func(p game.Player) game.UserID { return p.User })
	h.mu.Lock()
	h.live[id] = &liveGame{room: r, players: users, lastStep: lastStep}
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

func (h *Hub) touch(id game.GameID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if g, ok := h.live[id]; ok {
		g.lastStep = h.deps.Time.Now()
	}
}

func (h *Hub) liveRoom(id game.GameID) (*room.Room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	g, ok := h.live[id]
	if !ok {
		return nil, fmt.Errorf("no ongoing game %s: %w", id, store.ErrNotFound)
	}
	return g.room, nil
}

func (h *Hub) liveGamesOf(user game.UserID) []liveGame {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []liveGame
	for _, g := range h.live {
		if slices.Contains(g.players, user) {
			out = append(out, *g)
		}
	}
	return out
}

func (h *Hub) ongoingGames(ctx context.Context, user game.UserID) ([]protocol.GameUpdate, error) {
	type listed struct {
		update   protocol.GameUpdate
		lastStep time.Time
	}
	var games []listed
	for _, g := range h.liveGamesOf(user) {
		u, err := g.room.Snapshot(ctx, user)
		if err != nil {
			return nil, err
		}
		if u.Result == nil {
			games = append(games, listed{u, g.lastStep})
		}
	}
	slices.SortFunc(games, func(a, b listed) int {
		return cmp.Or(
			cmp.Compare(turnRank(a.update, user), turnRank(b.update, user)),
			b.lastStep.Compare(a.lastStep),
			cmp.Compare(a.update.GameID, b.update.GameID),
		)
	})
	return xslices.Map(games, func(g listed) protocol.GameUpdate { return g.update }), nil
}

func turnRank(u protocol.GameUpdate, user game.UserID) int {
	if slices.Contains(u.ToAct, user) {
		return 0
	}
	return 1
}

func (h *Hub) finishedGames(ctx context.Context, user game.UserID) ([]protocol.GameUpdate, error) {
	ids, err := h.deps.Store.FinishedGameIDs(ctx, user, h.deps.Limits.FinishedListLimit)
	if err != nil {
		return nil, err
	}
	return xslices.MapErr(ids, func(id game.GameID) (protocol.GameUpdate, error) {
		return h.replayedSnapshot(ctx, id, user)
	})
}

func (h *Hub) snapshot(ctx context.Context, id game.GameID, user game.UserID) (protocol.GameUpdate, error) {
	if r, err := h.liveRoom(id); err == nil {
		return r.Snapshot(ctx, user)
	}
	return h.replayedSnapshot(ctx, id, user)
}

// replayedSnapshot rebuilds a game that has no live room, then stops the
// rebuilt room so it never acts on its own.
func (h *Hub) replayedSnapshot(ctx context.Context, id game.GameID, user game.UserID) (protocol.GameUpdate, error) {
	rec, err := h.deps.Store.LoadGame(ctx, id)
	if err != nil {
		return protocol.GameUpdate{}, err
	}
	roomCtx, stop := context.WithCancel(h.ctx)
	defer stop()
	r, err := h.restore(roomCtx, rec)
	if err != nil {
		return protocol.GameUpdate{}, err
	}
	return r.Snapshot(ctx, user)
}

func (h *Hub) gameType(t game.GameType) (game.Game, error) {
	g, ok := h.gameTypes[t]
	if !ok {
		return nil, protocol.BadRequestf("unknown game %q", t)
	}
	return g, nil
}

func newSeed() game.Seed {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return game.Seed(binary.LittleEndian.Uint64(b[:]))
}
