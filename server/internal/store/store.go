package store

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"games.nvim/server/game"
	"games.nvim/server/internal/xslices"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

var (
	ErrNotFound       = errors.New("store: not found")
	ErrMissingOptions = errors.New("store: game options must be non-empty JSON")
)

type StepKind string

const (
	StepAction  StepKind = "action"
	StepControl StepKind = "control"
	StepTimeout StepKind = "timeout"
)

type Outcome string

const (
	OutcomeWin  Outcome = "win"
	OutcomeLoss Outcome = "loss"
	OutcomeDraw Outcome = "draw"
)

type StepRecord struct {
	Seq            game.Seq
	User           game.UserID
	Kind           StepKind
	Payload        json.RawMessage
	At             time.Time
	ClockRemaining map[game.UserID]time.Duration
}

type GameRecord struct {
	ID        game.GameID
	Type      game.GameType
	Options   json.RawMessage
	Seed      game.Seed
	Players   []game.Player
	Steps     []StepRecord
	CreatedAt time.Time
}

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

func dsn(path string) string {
	q := url.Values{}
	for _, pragma := range []string{
		"journal_mode(WAL)",
		"synchronous(NORMAL)",
		"busy_timeout(5000)",
		"foreign_keys(ON)",
	} {
		q.Add("_pragma", pragma)
	}
	return "file:" + path + "?" + q.Encode()
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) CreateUser(ctx context.Context, user game.UserID, passwordHash []byte) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		user, passwordHash, time.Now().UnixMilli())
	return err
}

func (s *Store) PasswordHash(ctx context.Context, user game.UserID) ([]byte, error) {
	var hash []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT password_hash FROM users WHERE username = ?`, user).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return hash, err
}

func (s *Store) UserExists(ctx context.Context, user game.UserID) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)`, user).Scan(&exists)
	return exists, err
}

func (s *Store) ListUsers(ctx context.Context) ([]game.UserID, error) {
	return queryAll(ctx, s.db, scanOne[game.UserID], `SELECT username FROM users ORDER BY username`)
}

func (s *Store) CreateGame(ctx context.Context, rec GameRecord) error {
	if len(rec.Options) == 0 {
		return ErrMissingOptions
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO games (game_id, game_type, options_json, rng_seed, created_at) VALUES (?, ?, ?, ?, ?)`,
			rec.ID, rec.Type, []byte(rec.Options), int64(rec.Seed), rec.CreatedAt.UnixMilli())
		if err != nil {
			return err
		}
		for _, p := range rec.Players {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO game_players (game_id, username, seat) VALUES (?, ?, ?)`,
				rec.ID, p.User, p.Seat); err != nil {
				return err
			}
		}
		for _, step := range rec.Steps {
			if err := insertStep(ctx, tx, rec.ID, step); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) AppendStep(ctx context.Context, id game.GameID, step StepRecord) error {
	return insertStep(ctx, s.db, id, step)
}

func (s *Store) FinishGame(ctx context.Context, id game.GameID, step StepRecord, reason game.ResultReason, outcomes map[game.UserID]Outcome) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := insertStep(ctx, tx, id, step); err != nil {
			return err
		}
		if err := requireAffected(ctx, tx, ErrNotFound,
			`UPDATE games SET finished_at = ?, result_reason = ? WHERE game_id = ?`,
			step.At.UnixMilli(), reason, id); err != nil {
			return err
		}
		for user, outcome := range outcomes {
			if err := requireAffected(ctx, tx, fmt.Errorf("store: outcome for %q: not a player of game %q", user, id),
				`UPDATE game_players SET outcome = ? WHERE game_id = ? AND username = ?`,
				outcome, id, user); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) LoadOngoing(ctx context.Context) ([]GameRecord, error) {
	return s.loadGames(ctx, `finished_at IS NULL`, `ORDER BY created_at, game_id`)
}

func (s *Store) LoadGame(ctx context.Context, id game.GameID) (GameRecord, error) {
	recs, err := s.loadGames(ctx, `game_id = ?`, ``, id)
	if err != nil {
		return GameRecord{}, err
	}
	if len(recs) == 0 {
		return GameRecord{}, ErrNotFound
	}
	return recs[0], nil
}

func (s *Store) FinishedGameIDs(ctx context.Context, user game.UserID, limit int) ([]game.GameID, error) {
	return queryAll(ctx, s.db, scanOne[game.GameID],
		`SELECT g.game_id FROM games g JOIN game_players p ON p.game_id = g.game_id
		 WHERE p.username = ? AND g.finished_at IS NOT NULL
		 ORDER BY g.finished_at DESC, g.game_id LIMIT ?`, user, limit)
}

// loadGames reads the games matching filter, then all their players and steps
// in one query each, selected by the same filter.
func (s *Store) loadGames(ctx context.Context, filter, order string, args ...any) ([]GameRecord, error) {
	recs, err := queryAll(ctx, s.db, scanGame,
		`SELECT game_id, game_type, options_json, rng_seed, created_at FROM games WHERE `+filter+` `+order, args...)
	if err != nil || len(recs) == 0 {
		return recs, err
	}
	ofMatchingGames := `game_id IN (SELECT game_id FROM games WHERE ` + filter + `)`
	players, err := queryAll(ctx, s.db, scanPlayer,
		`SELECT game_id, username, seat FROM game_players WHERE `+ofMatchingGames+` ORDER BY rowid`, args...)
	if err != nil {
		return nil, err
	}
	steps, err := queryAll(ctx, s.db, scanStep,
		`SELECT game_id, seq, username, kind, action_json, server_ts, clock_json
		 FROM actions WHERE `+ofMatchingGames+` ORDER BY game_id, seq`, args...)
	if err != nil {
		return nil, err
	}
	playersOf, stepsOf := groupByGame(players), groupByGame(steps)
	for i := range recs {
		recs[i].Players, recs[i].Steps = playersOf[recs[i].ID], stepsOf[recs[i].ID]
	}
	return recs, nil
}

type scanner interface {
	Scan(dest ...any) error
}

type ofGame[T any] struct {
	game  game.GameID
	value T
}

func groupByGame[T any](rows []ofGame[T]) map[game.GameID][]T {
	grouped := make(map[game.GameID][]T)
	for _, row := range rows {
		grouped[row.game] = append(grouped[row.game], row.value)
	}
	return grouped
}

func scanOne[T any](row scanner) (T, error) {
	var v T
	return v, row.Scan(&v)
}

func scanGame(row scanner) (GameRecord, error) {
	var rec GameRecord
	var seed, createdMS int64
	var options []byte
	err := row.Scan(&rec.ID, &rec.Type, &options, &seed, &createdMS)
	rec.Options, rec.Seed, rec.CreatedAt = options, game.Seed(seed), time.UnixMilli(createdMS)
	return rec, err
}

func scanPlayer(row scanner) (ofGame[game.Player], error) {
	var p ofGame[game.Player]
	return p, row.Scan(&p.game, &p.value.User, &p.value.Seat)
}

func scanStep(row scanner) (ofGame[StepRecord], error) {
	var s ofGame[StepRecord]
	var seq, atMS int64
	var payload, clock []byte
	if err := row.Scan(&s.game, &seq, &s.value.User, &s.value.Kind, &payload, &atMS, &clock); err != nil {
		return s, err
	}
	s.value.Seq, s.value.Payload, s.value.At = game.Seq(seq), payload, time.UnixMilli(atMS)
	if clock == nil {
		return s, nil
	}
	var remainingMS map[game.UserID]int64
	if err := json.Unmarshal(clock, &remainingMS); err != nil {
		return s, fmt.Errorf("store: decode clock: %w", err)
	}
	s.value.ClockRemaining = xslices.MapValues(remainingMS, func(ms int64) time.Duration { return time.Duration(ms) * time.Millisecond })
	return s, nil
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertStep(ctx context.Context, db execer, id game.GameID, step StepRecord) error {
	var clock []byte
	if step.ClockRemaining != nil {
		var err error
		if clock, err = json.Marshal(xslices.MapValues(step.ClockRemaining, time.Duration.Milliseconds)); err != nil {
			return fmt.Errorf("store: encode clock: %w", err)
		}
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO actions (game_id, seq, username, kind, action_json, server_ts, clock_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, int64(step.Seq), step.User, step.Kind, []byte(step.Payload), step.At.UnixMilli(), clock)
	return err
}

func (s *Store) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func queryAll[T any](ctx context.Context, db *sql.DB, scan func(scanner) (T, error), query string, args ...any) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func requireAffected(ctx context.Context, db execer, missing error, query string, args ...any) error {
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return missing
	}
	return nil
}
