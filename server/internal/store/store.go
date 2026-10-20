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
	Seq              game.Seq
	User             game.UserID
	Kind             StepKind
	Payload          json.RawMessage
	At               time.Time
	ClockRemainingMS map[game.UserID]int64
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

// Challenger relies on players being stored challenger first.
func (r GameRecord) Challenger() (game.UserID, bool) {
	if len(r.Players) == 0 {
		return "", false
	}
	return r.Players[0].User, true
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
	rows, err := s.db.QueryContext(ctx, `SELECT username FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	return collect(rows, func() (game.UserID, error) {
		var u game.UserID
		return u, rows.Scan(&u)
	})
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
		res, err := tx.ExecContext(ctx,
			`UPDATE games SET finished_at = ?, result_reason = ? WHERE game_id = ?`,
			step.At.UnixMilli(), reason, id)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return ErrNotFound
		}
		for user, outcome := range outcomes {
			res, err := tx.ExecContext(ctx,
				`UPDATE game_players SET outcome = ? WHERE game_id = ? AND username = ?`,
				outcome, id, user)
			if err != nil {
				return err
			}
			if n, err := res.RowsAffected(); err != nil {
				return err
			} else if n == 0 {
				return fmt.Errorf("store: outcome for %q: not a player of game %q", user, id)
			}
		}
		return nil
	})
}

func (s *Store) LoadOngoing(ctx context.Context) ([]GameRecord, error) {
	return s.loadGames(ctx, `WHERE finished_at IS NULL ORDER BY created_at, game_id`)
}

func (s *Store) LoadGame(ctx context.Context, id game.GameID) (GameRecord, error) {
	recs, err := s.loadGames(ctx, `WHERE game_id = ?`, id)
	if err != nil {
		return GameRecord{}, err
	}
	if len(recs) == 0 {
		return GameRecord{}, ErrNotFound
	}
	return recs[0], nil
}

func (s *Store) FinishedGameIDs(ctx context.Context, user game.UserID, limit int) ([]game.GameID, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.game_id FROM games g JOIN game_players p ON p.game_id = g.game_id
		 WHERE p.username = ? AND g.finished_at IS NOT NULL
		 ORDER BY g.finished_at DESC, g.game_id LIMIT ?`, user, limit)
	if err != nil {
		return nil, err
	}
	return collect(rows, func() (game.GameID, error) {
		var id game.GameID
		return id, rows.Scan(&id)
	})
}

func (s *Store) loadGames(ctx context.Context, where string, args ...any) ([]GameRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT game_id, game_type, options_json, rng_seed, created_at FROM games `+where, args...)
	if err != nil {
		return nil, err
	}
	recs, err := collect(rows, func() (GameRecord, error) {
		var rec GameRecord
		var seed, createdMS int64
		var options []byte
		err := rows.Scan(&rec.ID, &rec.Type, &options, &seed, &createdMS)
		rec.Options, rec.Seed, rec.CreatedAt = options, game.Seed(seed), time.UnixMilli(createdMS)
		return rec, err
	})
	if err != nil {
		return nil, err
	}
	for i := range recs {
		if recs[i].Players, err = s.loadPlayers(ctx, recs[i].ID); err != nil {
			return nil, err
		}
		if recs[i].Steps, err = s.loadSteps(ctx, recs[i].ID); err != nil {
			return nil, err
		}
	}
	return recs, nil
}

func (s *Store) loadPlayers(ctx context.Context, id game.GameID) ([]game.Player, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT username, seat FROM game_players WHERE game_id = ? ORDER BY rowid`, id)
	if err != nil {
		return nil, err
	}
	return collect(rows, func() (game.Player, error) {
		var p game.Player
		return p, rows.Scan(&p.User, &p.Seat)
	})
}

func (s *Store) loadSteps(ctx context.Context, id game.GameID) ([]StepRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT seq, username, kind, action_json, server_ts, clock_json
		 FROM actions WHERE game_id = ? ORDER BY seq`, id)
	if err != nil {
		return nil, err
	}
	return collect(rows, func() (StepRecord, error) {
		var step StepRecord
		var seq, atMS int64
		var payload, clock []byte
		if err := rows.Scan(&seq, &step.User, &step.Kind, &payload, &atMS, &clock); err != nil {
			return step, err
		}
		step.Seq, step.Payload, step.At = game.Seq(seq), payload, time.UnixMilli(atMS)
		if clock != nil {
			if err := json.Unmarshal(clock, &step.ClockRemainingMS); err != nil {
				return step, fmt.Errorf("store: decode clock: %w", err)
			}
		}
		return step, nil
	})
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertStep(ctx context.Context, db execer, id game.GameID, step StepRecord) error {
	var clock []byte
	if step.ClockRemainingMS != nil {
		var err error
		if clock, err = json.Marshal(step.ClockRemainingMS); err != nil {
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

func collect[T any](rows *sql.Rows, scan func() (T, error)) ([]T, error) {
	defer rows.Close()
	var out []T
	for rows.Next() {
		v, err := scan()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
