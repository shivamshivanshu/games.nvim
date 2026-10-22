package store

import (
	"cmp"
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

var (
	ErrNotFound       = errors.New("store: not found")
	ErrMissingOptions = errors.New("store: game options must be non-empty JSON")
	ErrNotFinished    = errors.New("store: game is not finished")
	ErrNotOngoing     = errors.New("store: no ongoing game with that id")
	ErrUserExists     = errors.New("store: user already exists")
)

type StepKind string

const (
	StepAction  StepKind = "action"
	StepControl StepKind = "control"
	StepTimeout StepKind = "timeout"
	// StepTick and StepPause are the core's steps of a game.Ticking game,
	// made by no player.
	StepTick  StepKind = "tick"
	StepPause StepKind = "pause"
)

type Outcome string

const (
	OutcomeWin  Outcome = "win"
	OutcomeLoss Outcome = "loss"
	OutcomeDraw Outcome = "draw"
	// OutcomeNone is an aborted game's: it counts for nobody.
	OutcomeNone Outcome = ""
)

type StepRecord struct {
	Seq            game.Seq
	User           game.UserID
	Kind           StepKind
	Payload        json.RawMessage
	At             time.Time
	ClockRemaining map[game.UserID]time.Duration
}

type Standing struct {
	Outcome Outcome
	Score   int64
}

type Ending struct {
	Kind      game.ResultKind
	Reason    game.ResultReason
	Standings map[game.UserID]Standing
}

// Finish is all that is kept of a game once it ends: its steps are deleted,
// and each player's final view and the game's exports stay in their place.
type Finish struct {
	At  time.Time
	Seq game.Seq
	Ending
	Summary []byte
	Digest  game.Digest
	Views   map[game.UserID]json.RawMessage
	Exports map[string]string
}

type GameRecord struct {
	ID        game.GameID
	Type      game.GameType
	Options   json.RawMessage
	Seed      game.Seed
	Players   []game.Player
	Steps     []StepRecord
	CreatedAt time.Time
	Finish    *Finish
}

// Store writes through one connection and reads through a pool: under WAL,
// readers see the last commit without waiting on the writer. A database with
// no file of its own exists only inside its connection, so it reads through
// the writer.
type Store struct {
	writer     *sql.DB
	reader     *sql.DB
	insertStep *sql.Stmt
	// afterResolve runs between resolving loadGames' ids and reading them; tests use it.
	afterResolve func()
}

var (
	writerPragmas = []string{"journal_mode(WAL)", "synchronous(NORMAL)", "busy_timeout(5000)", "foreign_keys(ON)"}
	readerPragmas = []string{"busy_timeout(5000)", "query_only(ON)"}
)

func Open(ctx context.Context, path string) (*Store, error) {
	writer, err := sql.Open("sqlite", dsn(path, writerPragmas))
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	writer.SetMaxOpenConns(1)
	if _, err := writer.ExecContext(ctx, schema); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("store: apply schema: %w", err)
	}
	insertStep, err := writer.PrepareContext(ctx,
		`INSERT INTO actions (game_id, seq, username, kind, action_json, server_ts, clock_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("store: prepare step insert: %w", err)
	}
	if !hasFile(path) {
		return &Store{writer: writer, reader: writer, insertStep: insertStep}, nil
	}
	reader, err := sql.Open("sqlite", dsn(path, readerPragmas))
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("store: open %s for reading: %w", path, err)
	}
	return &Store{writer: writer, reader: reader, insertStep: insertStep}, nil
}

func hasFile(path string) bool { return path != "" && path != ":memory:" }

func dsn(path string, pragmas []string) string {
	q := url.Values{}
	for _, pragma := range pragmas {
		q.Add("_pragma", pragma)
	}
	return "file:" + path + "?" + q.Encode()
}

func (s *Store) Close() error {
	closed := s.insertStep.Close()
	if s.reader == s.writer {
		return errors.Join(closed, s.writer.Close())
	}
	return errors.Join(closed, s.reader.Close(), s.writer.Close())
}

// CreateUser refuses a name game.CheckUserID rejects, and is ErrUserExists
// for a name already taken.
func (s *Store) CreateUser(ctx context.Context, user game.UserID, passwordHash []byte) error {
	if err := game.CheckUserID(user); err != nil {
		return err
	}
	return requireAffected(ctx, s.writer, ErrUserExists,
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?) ON CONFLICT (username) DO NOTHING`,
		user, passwordHash, time.Now().UnixMilli())
}

func (s *Store) PasswordHash(ctx context.Context, user game.UserID) ([]byte, error) {
	var hash []byte
	err := s.reader.QueryRowContext(ctx,
		`SELECT password_hash FROM users WHERE username = ?`, user).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return hash, err
}

func (s *Store) UserExists(ctx context.Context, user game.UserID) (bool, error) {
	var exists bool
	err := s.reader.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)`, user).Scan(&exists)
	return exists, err
}

func (s *Store) ListUsers(ctx context.Context) ([]game.UserID, error) {
	return queryAll(ctx, s.reader, scanOne[game.UserID], `SELECT username FROM users ORDER BY username`)
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
		return insertSteps(ctx, tx.StmtContext(ctx, s.insertStep), rec.ID, rec.Steps)
	})
}

// AppendSteps stores steps in one transaction: all of them or none.
func (s *Store) AppendSteps(ctx context.Context, id game.GameID, steps ...StepRecord) error {
	return s.inTx(ctx, func(tx *sql.Tx) error { return insertSteps(ctx, tx.StmtContext(ctx, s.insertStep), id, steps) })
}

// FinishGame stores a game's ending and summary and deletes its steps.
func (s *Store) FinishGame(ctx context.Context, id game.GameID, f Finish) error {
	exports, err := json.Marshal(f.Exports)
	if err != nil {
		return fmt.Errorf("store: encode exports: %w", err)
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := requireAffected(ctx, tx, ErrNotOngoing,
			`UPDATE games SET finished_at = ?, final_seq = ?, result_kind = ?, result_reason = ?, summary = ?, digest = ?, exports_json = ?
			 WHERE game_id = ? AND finished_at IS NULL`,
			f.At.UnixMilli(), int64(f.Seq), string(f.Kind), f.Reason, f.Summary, f.Digest, exports, id); err != nil {
			return err
		}
		for user, standing := range f.Standings {
			outcome := sql.Null[Outcome]{V: standing.Outcome, Valid: standing.Outcome != OutcomeNone}
			if err := requireAffected(ctx, tx, fmt.Errorf("store: standing for %q: not a player of game %q", user, id),
				`UPDATE game_players SET outcome = ?, score = ?, final_view = ? WHERE game_id = ? AND username = ?`,
				outcome, standing.Score, []byte(f.Views[user]), id, user); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM actions WHERE game_id = ?`, id)
		return err
	})
}

func (s *Store) Digest(ctx context.Context, id game.GameID) (game.Digest, error) {
	digest, err := scanOne[sql.Null[game.Digest]](s.reader.QueryRowContext(ctx,
		`SELECT digest FROM games WHERE game_id = ?`, id))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", ErrNotFound
	case err != nil:
		return "", err
	case !digest.Valid:
		return "", ErrNotFinished
	}
	return digest.V, nil
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

// LoadFinished reads the user's most recently finished games, newest first.
func (s *Store) LoadFinished(ctx context.Context, user game.UserID, limit int) ([]GameRecord, error) {
	return s.loadGames(ctx,
		`game_id IN (SELECT g.game_id FROM games g JOIN game_players p ON p.game_id = g.game_id
		  WHERE p.username = ? AND g.finished_at IS NOT NULL
		  ORDER BY g.finished_at DESC, g.game_id LIMIT ?)`,
		`ORDER BY finished_at DESC, game_id`, user, limit)
}

// loadGames resolves the ids matching filter once, then reads exactly those
// games with their players, and the steps of the ones still ongoing.
func (s *Store) loadGames(ctx context.Context, filter, order string, args ...any) ([]GameRecord, error) {
	ids, err := s.resolveIDs(ctx, filter, order, args...)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	recs, players, err := s.loadRows(ctx, ids)
	if err != nil {
		return nil, err
	}
	steps, err := s.ongoingSteps(ctx, recs)
	if err != nil {
		return nil, err
	}
	return assemble(recs, players, steps), nil
}

func (s *Store) resolveIDs(ctx context.Context, filter, order string, args ...any) ([]game.GameID, error) {
	ids, err := queryAll(ctx, s.reader, scanOne[game.GameID], `SELECT game_id FROM games WHERE `+filter+` `+order, args...)
	if err == nil && len(ids) > 0 && s.afterResolve != nil {
		s.afterResolve()
	}
	return ids, err
}

// loadRows reads the game rows of ids, in the order of ids, and their player rows.
func (s *Store) loadRows(ctx context.Context, ids []game.GameID) ([]GameRecord, []ofGame[playerRow], error) {
	recs, err := queryAll(ctx, s.reader, scanGame,
		`SELECT game_id, game_type, options_json, rng_seed, created_at,
		        finished_at, final_seq, result_kind, result_reason, summary, digest, exports_json
		 FROM games WHERE `+inSet("game_id", len(ids)), xslices.Any(ids)...)
	if err != nil {
		return nil, nil, err
	}
	position := make(map[game.GameID]int, len(ids))
	for i, id := range ids {
		position[id] = i
	}
	slices.SortFunc(recs, func(a, b GameRecord) int { return cmp.Compare(position[a.ID], position[b.ID]) })
	players, err := queryAll(ctx, s.reader, scanPlayer,
		`SELECT game_id, username, seat, outcome, score, final_view FROM game_players WHERE `+inSet("game_id", len(ids))+` ORDER BY rowid`, xslices.Any(ids)...)
	if err != nil {
		return nil, nil, err
	}
	return recs, players, nil
}

// assemble fills each record with its players, its steps, and, once it is
// finished, each player's standing and final view.
func assemble(recs []GameRecord, players []ofGame[playerRow], steps []ofGame[StepRecord]) []GameRecord {
	playersOf, stepsOf := groupByGame(players), groupByGame(steps)
	for i := range recs {
		recs[i].Steps = stepsOf[recs[i].ID]
		for _, row := range playersOf[recs[i].ID] {
			recs[i].Players = append(recs[i].Players, row.player)
			if finish := recs[i].Finish; finish != nil {
				finish.Standings[row.player.User] = Standing{Outcome: row.outcome.V, Score: row.score.V}
				if row.view != nil {
					finish.Views[row.player.User] = row.view
				}
			}
		}
	}
	return recs
}

func (s *Store) ongoingSteps(ctx context.Context, recs []GameRecord) ([]ofGame[StepRecord], error) {
	var ongoing []game.GameID
	for _, rec := range recs {
		if rec.Finish == nil {
			ongoing = append(ongoing, rec.ID)
		}
	}
	if len(ongoing) == 0 {
		return nil, nil
	}
	return queryAll(ctx, s.reader, scanStep,
		`SELECT game_id, seq, username, kind, action_json, server_ts, clock_json
		 FROM actions WHERE `+inSet("game_id", len(ongoing))+` ORDER BY game_id, seq`, xslices.Any(ongoing)...)
}

func inSet(column string, n int) string {
	return column + ` IN (?` + strings.Repeat(`, ?`, n-1) + `)`
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
	var finishedMS, finalSeq sql.Null[int64]
	var kind, reason sql.Null[string]
	var summary, exports []byte
	var digest sql.Null[game.Digest]
	if err := row.Scan(&rec.ID, &rec.Type, &options, &seed, &createdMS,
		&finishedMS, &finalSeq, &kind, &reason, &summary, &digest, &exports); err != nil {
		return rec, err
	}
	rec.Options, rec.Seed, rec.CreatedAt = options, game.Seed(seed), time.UnixMilli(createdMS)
	if !finishedMS.Valid {
		return rec, nil
	}
	finish := &Finish{
		At:      time.UnixMilli(finishedMS.V),
		Seq:     game.Seq(finalSeq.V),
		Ending:  Ending{Kind: game.ResultKind(kind.V), Reason: game.ResultReason(reason.V), Standings: map[game.UserID]Standing{}},
		Summary: summary,
		Digest:  digest.V,
		Views:   map[game.UserID]json.RawMessage{},
	}
	if len(exports) > 0 {
		if err := json.Unmarshal(exports, &finish.Exports); err != nil {
			return rec, fmt.Errorf("store: game %s exports: %w", rec.ID, err)
		}
	}
	if finish.Exports == nil {
		finish.Exports = map[string]string{}
	}
	rec.Finish = finish
	return rec, nil
}

type playerRow struct {
	player  game.Player
	outcome sql.Null[Outcome]
	score   sql.Null[int64]
	view    json.RawMessage
}

func scanPlayer(row scanner) (ofGame[playerRow], error) {
	var p ofGame[playerRow]
	var view []byte
	err := row.Scan(&p.game, &p.value.player.User, &p.value.player.Seat, &p.value.outcome, &p.value.score, &view)
	p.value.view = view
	return p, err
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

func insertSteps(ctx context.Context, insert *sql.Stmt, id game.GameID, steps []StepRecord) error {
	for _, step := range steps {
		var clock []byte
		if step.ClockRemaining != nil {
			var err error
			if clock, err = json.Marshal(xslices.MapValues(step.ClockRemaining, time.Duration.Milliseconds)); err != nil {
				return fmt.Errorf("store: encode clock: %w", err)
			}
		}
		if _, err := insert.ExecContext(ctx, id, int64(step.Seq), step.User, step.Kind, []byte(step.Payload), step.At.UnixMilli(), clock); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.writer.BeginTx(ctx, nil)
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
	defer func() { _ = rows.Close() }()
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
