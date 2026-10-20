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
	id, err := h.startGame(c)
	if err != nil {
		slog.Error("hub: start game", "challenge", c.ChallengeID, "err", err)
	}
	return id, err
}

func (h *Hub) startGame(c protocol.Challenge) (game.GameID, error) {
	g, state, err := h.prepareGame(c)
	if err != nil {
		return "", err
	}
	if err := h.persistNewGame(g); err != nil {
		return "", err
	}
	h.launch(g, state)
	return g.ID, nil
}

func (h *Hub) prepareGame(c protocol.Challenge) (room.Setup, game.ErasedState, error) {
	g, err := h.gameType(c.Game)
	if err != nil {
		return room.Setup{}, nil, err
	}
	seed := newSeed()
	state, seats, err := g.New(c.Options, c.From, c.To, game.StepRNG(seed, 0))
	if err != nil {
		return room.Setup{}, nil, err
	}
	players, err := seated(c.Participants(), seats)
	if err != nil {
		return room.Setup{}, nil, err
	}
	setup, err := h.roomSetup(room.Params{
		ID:         game.GameID(uuid.NewV4()),
		Game:       g,
		Options:    c.Options,
		Seed:       seed,
		Challenger: c.From,
		Players:    players,
		CreatedAt:  h.deps.Time.Now(),
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

func (h *Hub) launch(g room.Setup, state game.ErasedState) {
	r := room.Start(h.ctx, g, state)
	h.track(g.ID, r, g.Players)
	h.sendCurrent(r, g.Players)
}

// seated keeps the challenger first, which challengerOf relies on after a restart.
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

func challengerOf(players []game.Player) game.UserID {
	if len(players) == 0 {
		return ""
	}
	return players[0].User
}

func (h *Hub) sendCurrent(r *room.Room, players []game.Player) {
	for _, p := range players {
		s, err := r.Snapshot(h.ctx, p.User)
		if err != nil {
			slog.Error("hub: initial update", "user", p.User, "err", err)
			continue
		}
		h.SendUpdate(p.User, s.Update)
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
		h.track(rec.ID, r, rec.Players)
	}
	return nil
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
	setup, err := h.roomSetup(room.Params{
		ID:         rec.ID,
		Game:       g,
		Options:    opts,
		Seed:       rec.Seed,
		Challenger: challengerOf(rec.Players),
		Players:    rec.Players,
		CreatedAt:  rec.CreatedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("hub: restore %s: %w", rec.ID, err)
	}
	return room.Restore(ctx, setup, rec.Steps)
}

func (h *Hub) roomSetup(p room.Params) (room.Setup, error) {
	p.Persist, p.Broadcast, p.Time = h.deps.Store, h, h.deps.Time
	return room.NewSetup(p)
}

func (h *Hub) track(id game.GameID, r *room.Room, players []game.Player) {
	users := xslices.Map(players, func(p game.Player) game.UserID { return p.User })
	h.mu.Lock()
	h.live[id] = &liveGame{room: r, players: users}
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
		s, err := r.Snapshot(ctx, user)
		return s.Update, err
	}
	return h.replayedSnapshot(ctx, id, user)
}

// replayedSnapshot rebuilds a game that has no live room, then stops the
// rebuilt room so it never acts on its own.
func (h *Hub) replayedSnapshot(ctx context.Context, id game.GameID, user game.UserID) (protocol.GameUpdate, error) {
	rec, err := h.deps.Store.LoadGame(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return protocol.GameUpdate{}, protocol.Error{Code: protocol.CodeNotFound, Msg: err.Error()}
	}
	if err != nil {
		return protocol.GameUpdate{}, err
	}
	roomCtx, stop := context.WithCancel(h.ctx)
	defer stop()
	r, err := h.restore(roomCtx, rec)
	if err != nil {
		return protocol.GameUpdate{}, err
	}
	s, err := r.Snapshot(ctx, user)
	return s.Update, err
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
