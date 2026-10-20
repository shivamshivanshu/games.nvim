CREATE TABLE IF NOT EXISTS users (
    username      TEXT PRIMARY KEY,
    password_hash BLOB NOT NULL,
    created_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS games (
    game_id       TEXT PRIMARY KEY,
    game_type     TEXT NOT NULL,
    options_json  BLOB NOT NULL,
    rng_seed      INTEGER NOT NULL,
    result_reason TEXT,
    created_at    INTEGER NOT NULL,
    finished_at   INTEGER,
    final_seq     INTEGER,
    result_kind   TEXT CHECK (result_kind IN ('decisive', 'draw', 'aborted')),
    summary       BLOB,
    digest        TEXT
);

CREATE TABLE IF NOT EXISTS game_players (
    game_id  TEXT NOT NULL REFERENCES games(game_id),
    username TEXT NOT NULL REFERENCES users(username),
    seat     TEXT NOT NULL,
    outcome  TEXT CHECK (outcome IN ('win', 'loss', 'draw')),
    score    INTEGER,
    PRIMARY KEY (game_id, username)
);

CREATE TABLE IF NOT EXISTS actions (
    game_id     TEXT NOT NULL REFERENCES games(game_id),
    seq         INTEGER NOT NULL,
    username    TEXT NOT NULL,
    kind        TEXT NOT NULL,
    action_json BLOB,
    server_ts   INTEGER NOT NULL,
    clock_json  BLOB,
    PRIMARY KEY (game_id, seq)
);

CREATE INDEX IF NOT EXISTS game_players_username ON game_players(username);
CREATE INDEX IF NOT EXISTS games_finished_at ON games(finished_at);
