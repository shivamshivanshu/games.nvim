# games.nvim wire protocol

This is the contract between the games server (`server/`) and any client. A
client needs only this document, the JSON Schema under [`schema/`](schema/),
and the transcripts under [`conformance/`](conformance/). The server sends game
state and everything derived from it: legal moves, check squares, ranked
leaderboards and export files. A client presents that data and re-implements no
game rules. Display wording, glyphs and colours stay client-side, so this
document lists the enums behind them.

Protocol version: **1**. It is sent in `auth.login` as `v`.

## Files

| Path | What it holds |
|---|---|
| `schema/protocol.json` | JSON Schema (draft 2020-12) of one envelope. `$defs` holds every body; `x-messages` maps each message type to `{request, reply}` or `{push}` refs; `x-version` is the protocol version. |
| `schema/games/<game>.json` | One file per registered game. `x-action`, `x-view` and `x-events` (kind → ref) point into its `$defs`. `ResultReason` lists the reasons the game adds; `ActionKind` lists its action kinds. Core enums are referred to as `../protocol.json#/$defs/...`. |
| `conformance/*.ndjson` | Replayable transcripts; see [Conformance](#conformance). |

The schema is generated from the Go types: `make schema` writes it, and
`make check` fails when the checked-in copy differs.

## Framing

- TCP. Each message is one JSON object on one line, UTF-8, ending in `\n`. Blank lines are ignored.
- A line is at most 64 KiB. A longer line gets a `bad_request` error with no `id`, and the server then closes the connection.
- Each side writes whole lines only. The server never splits a message across lines.

## Envelope

```json
{"id": 7, "type": "game.act", "body": {...}}
```

| Field | Rule |
|---|---|
| `id` | An unsigned integer the client chooses per request; every request must carry one. The server echoes it on the reply. Pushes have no `id`. |
| `type` | The message type; see [Requests](#requests) and [Pushes](#pushes). Replies are `ok` or `error`. |
| `body` | An object. A request may omit it, which means `{}`; the schema marks it optional for that reason. The server always sends it. |

- Each request gets exactly one reply: `ok` with the reply body, or `error` with `{code, message}`.
- Replies on one connection come in request order. Pushes can arrive between them, including before the reply to the request that caused them.
- A line that is not a valid envelope, or a request without an `id`, gets an `error` with no `id` (`bad_request`) and is not acted on. The connection stays open.
- When the server is at its connection limit (256), it writes one `error` with no `id`, code `internal` and message `server full`, before any login, and then closes the connection.
- `message` is human-readable English. Clients branch on `code` only.

## Session

1. Connect, then send `auth.login` within **10 s**, or the server closes the connection.
2. Before login, any other request gets `unauthenticated`.
3. After login, send something (`ping` will do) at least every `heartbeat_ms` (15000). The server closes a connection that sends nothing for **3 × heartbeat** (45 s). The server does not ping the client.
4. One user may hold several connections. Every connection gets every push for that user.
5. A connection that does not read its pushes is closed once 64 frames queue up for it. The client then reconnects and resyncs.

## Auth and lockout

- `auth.login` takes `{v, username, password}`. Passwords travel in clear text, so run the server on localhost or behind a TLS tunnel.
- When `v` differs from the server's version, the reply is `version_mismatch`. This is fatal: do not retry.
- A wrong username or password gets `auth_failed`. **Lockout is per client IP**: after 5 failures within 5 minutes, that IP gets `auth_failed` for 5 minutes, even with the right password. A client stops retrying on `auth_failed`.
- A second `auth.login` on a logged-in connection gets `bad_request`.

## Requests

Every body here is defined in `schema/protocol.json`. The "Game update" type
is defined in [Game updates](#game-updates).

| Type | Body | `ok` reply | Notes |
|---|---|---|---|
| `auth.login` | `LoginRequest {v, username, password}` | `LoginResult {user, heartbeat_ms}` | Only allowed before login. |
| `ping` | `{}` | `{}` | Heartbeat. |
| `catalog.list` | `{}` | `{games: GameInfo[]}` | The registered games; see [Catalogue](#catalogue). |
| `user.list` | `{}` | `{users: [{user, online}]}` | Sorted by name. |
| `challenge.create` | `{to: user[], game, options}` | `{challenge_id}` | Pushes `challenge.changed` (pending) to every participant. |
| `challenge.accept` | `{challenge_id}` | `{}` | Invitees only. Once every invitee has accepted, the game starts; see [Challenges](#challenges). |
| `challenge.decline` | `{challenge_id}` | `{}` | Invitees only. Resolves the challenge as `declined`. |
| `challenge.cancel` | `{challenge_id}` | `{}` | The challenger only. Resolves it as `cancelled`. |
| `challenge.list` | `{}` | `{challenges: Challenge[]}` | Pending challenges the user is in, soonest expiry first. |
| `game.list` | `{status?: "ongoing" \| "finished"}` | `{games: GameUpdate[]}` | The default is `ongoing`: the user's turn first, then the most recent step. `finished` gives the 50 most recently finished. |
| `game.snapshot` | `{game_id}` | `GameUpdate` | The game as the user sees it now. Players only; others get `not_found`. |
| `game.act` | `{game_id, seq, action}` | `{seq}` | `action` is the game's own; see `x-action` and the [Action catalogue](#action-catalogue). |
| `game.control` | `{game_id, seq, control}` | `{seq}` | `control` is one of the update's `controls`. |
| `game.start` | `{game, options}` | `GameUpdate` (seq 0) | Solo games only (`players.max == 1`). The first update is also pushed. |
| `stats.get` | `{user?}` | `StatsResult` | See [Stats](#stats). |
| `game.verify` | `{summary}` | `{}` | Checks an archived summary; see [Finished games](#finished-games). |

## Pushes

| Type | Body | Sent to |
|---|---|---|
| `challenge.changed` | `Challenge` | Every participant, on each change: created, an accept, accepted, declined, cancelled, expired. |
| `game.update` | `GameUpdate` | Every player, after every step (an action, a control or a timeout). Each recipient gets its own `view` and `controls`. |

## Error codes

| Code | When |
|---|---|
| `bad_request` | Malformed envelope or body, a request without `id`, unknown type, unknown game, user, option or `status`, an invalid option value, an invalid invitee list (empty, the wrong size for the game, a duplicate, or the challenger), `game.start` of a game that is not solo, an unknown or already resolved challenge, a challenge the user may not resolve, an action the game cannot parse, a second `auth.login`. |
| `version_mismatch` | `auth.login` with another protocol version. |
| `unauthenticated` | Any request but `auth.login` before login. |
| `auth_failed` | Wrong credentials, or the IP is locked out. |
| `not_found` | No such game, the user is not one of its players, or `game.act`/`game.control` on a finished game the server has dropped (see below). |
| `not_your_turn` | `game.act` when the user is not in `to_act`. |
| `illegal_action` | The game rejects the action, the control is not in `controls`, or the game is over but the server still holds it. |
| `stale` | `seq` is not the game's current seq. Nothing changed. |
| `internal` | A server fault, or `server full` before login. |
| `summary_mismatch` | `game.verify` with a summary the server did not seal. |

The client also uses `disconnected` for a lost connection. The server never
sends it.

## Game updates

`GameUpdate` is the whole game as one player sees it. It is complete, never a
delta.

| Field | Meaning |
|---|---|
| `game_id` | A UUID. |
| `game` | The game type, e.g. `chess`. |
| `options` | The resolved options: every option the game has, with defaults filled in. |
| `seq` | See [Seq rules](#seq-rules). |
| `players` | `[{user, seat}]`, in participant order: the challenger first, then the invitees in the challenge's `to` order. Seats are game-defined strings. |
| `to_act` | The users who may act now. It is empty once the game is finished. |
| `result` | `null` while ongoing; otherwise `{kind, winners: user[], reason}`. |
| `clock` | `null` for a game without a clock; see [Clock](#clock). |
| `draw_agreed` | The users who offered a draw that is still pending. |
| `controls` | The controls this recipient may send now, from `resign`, `offer_draw`, `decline_draw` and `abort`. `abort` is offered only until every player has acted once. |
| `events` | The events of the step this update reports. Each is an object with a `kind`; see `x-events`. Snapshots and later reads have `[]`. |
| `view` | The game's view as this recipient sees it; see `x-view`. A finished game keeps each player's final view. |
| `started_at` | Unix ms when the game started. |
| `finished_at`, `summary`, `digest`, `exports` | `null` until finished; see [Finished games](#finished-games). |

Arrays are never `null`. Every field is always present.

### Seq rules

- `seq` counts the game's steps: 0 at the start, plus 1 for each accepted action, control or timeout.
- `game.act` and `game.control` carry the seq the client acted on. If it differs from the current seq, the reply is `stale` and nothing changes. The reply's `seq` is the step's new seq, and the matching `game.update` push carries the same seq.
- Updates with equal seq hold the same state. Keep the update with the highest seq, and ignore any update whose seq is not higher than the one held.
- A push whose seq jumps by more than 1 means steps were missed. Their events are lost, so take a `game.snapshot` rather than relying on the push.
- Error-checking order for `game.act` and `game.control`: `not_found`, then `stale`, then the game's verdict (`not_your_turn`, `illegal_action`, `bad_request`).
- On a finished game they get `illegal_action` for a short while after the finishing step, then `not_found` once the server drops the game. Treat either as "the game is over" and take a `game.snapshot`.

### Clock

`{remaining_ms: {user: ms}, running: user | null, as_of_ms}`. This is the
clock as it stood at `as_of_ms`. On a push, `as_of_ms` is the step's time; on a
snapshot or list, it is the time of the read.

- `running` is the user whose time is falling, or `null` when no clock runs. A clock runs only once every player has acted once.
- To show a countdown, compute `remaining_ms[running] - max(0, now_ms - as_of_ms)`, clamped at 0. This assumes the client's and the server's wall clocks agree. The server alone decides timeouts, and sends them as a step whose result reason is `timeout`.
- The increment is added to the mover after their move.

### Finished games

- The finishing update carries:
  - `result`;
  - `finished_at`;
  - `summary`: a JSON text of `{game_id, game, started_at, finished_at, reason, outcomes: [{user, score}]}`;
  - `digest`: the lowercase hex SHA-256 of the summary's exact bytes;
  - `exports`: `{filename: content}`, the files the game records itself in. Chess sends `game.pgn`.
- Later reads (`game.snapshot`, `game.list` with `finished`, a reconnect) give the same update, including each player's final `view` and `exports`, but with `events: []` and `clock: null`.
- `game.verify {summary}` replies `ok` only for byte-identical summary text the server sealed. Exports are not covered by the digest.
- A client archive writes `exports` verbatim. The file names are the game's own.

## Challenges

`Challenge {challenge_id, from, to, accepted, game, options, state, by, game_id, expires_at}`

- `state` is one of `pending`, `accepted`, `declined`, `cancelled`, `expired`. Every state but `pending` is final. Ignore any `pending` push for a challenge already resolved.
- `by` is the user who declined or cancelled; otherwise `null`.
- `game_id` is set once the challenge is `accepted`.
- `expires_at` is unix ms, 5 minutes after creation.
- When the last invitee accepts, every participant gets `challenge.changed` with state `accepted` and the `game_id`, and every player gets the first `game.update` (seq 0). The two can arrive in either order.

## Catalogue

`GameInfo {id, name, players: {min, max}, capabilities, options, actions}`

- `capabilities` come from `resign`, `draw`, `abort` and `clock`. They gate the matching controls.
- `options` is `{name: {type, values, default, max_len?, role?}}`:
  - `type: "enum"`: the value is one of `values`.
  - `type: "text"`: free text of at most `max_len` bytes. `values` is `[]`, and control and format characters are refused.
  - `role` marks an option whose meaning the core knows. `role: "clock"` is the time control. Find it by role, not by name.

### Options grammar

- `options` in `challenge.create` and `game.start` is an object of **string** values. Any option left out takes its default; an unknown name, a value that is not a string (`null` included) or a value outside `values` gets `bad_request`.
- The time control is `"M+S"`, meaning M minutes plus an S-second increment (e.g. `"10+5"`), or `"none"`. The catalogue lists the allowed values.
- `first` (in games that have it) is relative to the challenger: `me` means the challenger moves first, `opponent` means the invitee does, `random` picks one. Chess uses `color` (`white`, `black` or `random`) for the challenger's colour instead.

### Action catalogue

`actions: [{kind, description, input}]` lists every action the game accepts.
An action object always carries `kind`.

- `input` is `null` for an action that is its kind alone, e.g. chess `{"kind": "claim_draw"}`.
- Otherwise, `input` is `{field, description, pattern?, values}`: the action takes one text field named `field`. Its text matches `pattern` (RE2 syntax) or is one of `values`, never both. For example, chess `move` has the field `move`: `{"kind": "move", "move": "Nf3"}`.
- Whether an action is available now is in the view, e.g. chess `claimable_draw`.

## Open enums

- Result `kind`: `decisive`, `draw` or `aborted`.
- Result `reason` is open:
  - The core sends `resign`, `timeout`, `draw_agreed` and `aborted`.
  - Each game adds its own; see `ResultReason` in its schema.
  - A client shows an unknown reason as its raw string.
- Event `kind` is per game; see `x-events`.

### Chess

- Reasons: `checkmate`, `stalemate`, `insufficient_material`, `fivefold_repetition`, `seventy_five_move`, `threefold_repetition`, `fifty_move`.
- Events: `moved {by: seat, san}`.
- Seats: `white`, `black`.
- View `{fen, history_san, last_move: {from, to} | null, legal_moves, claimable_draw, check_square}`:
  - `legal_moves` is in UCI (`e2e4`, `e7e8q`). It holds the viewer's own moves: it is empty unless the viewer is to move.
  - `claimable_draw` is true when the viewer, being to move, may claim a threefold or fifty-move draw with `claim_draw`.
  - `check_square` is the square of the king of the side to move while it is in check; otherwise `null`.
- Actions: `move` (`move`: SAN or UCI, any case) and `claim_draw`.
- Export `game.pgn`:
  - the Seven Tag Roster; `Date` is the day the game **started, in UTC**;
  - `SetUp`/`FEN` tags when the game started from a position other than the standard one;
  - move numbers follow the start position (`12...` when Black moves first);
  - lines wrap at 79 columns.
- Material display (a client convention): count P=1, N=3, B=3, R=5, Q=9, and show only the imbalance, the surplus of the side ahead, never both totals.

## Stats

`stats.get {user?}` → `{records, head_to_head, leaderboard}`, already ranked
by the server.

- A rank orders most wins, then fewest losses, then most draws, then user name.
- `records`: `[{user, game, wins, losses, draws}]`, grouped by game, ranked within each game.
- `head_to_head`: `[{user, opponent, game, wins, losses, draws}]`, by user, opponent, game.
- `leaderboard`: `[{user, wins, losses, draws}]`, totals over every game, ranked.
- `user` limits `records` and `head_to_head` to that user; a user with no finished games gets them empty. `leaderboard` is never filtered.

## Times

Every instant on the wire is **unix milliseconds** (an integer):
`started_at`, `finished_at`, `expires_at` and `as_of_ms`. Every duration is
milliseconds: `heartbeat_ms` and `remaining_ms`.

## Client duties

- **Reconnect** on a lost connection:
  - use exponential backoff with jitter (the Neovim client uses 1 s doubling to 30 s, ±20%);
  - do not reconnect after `auth_failed` or `version_mismatch`.
- **Resync** after every login, because pushes sent while disconnected are lost:
  1. `game.list` (ongoing) replaces the cached state of every listed game.
  2. A `game.snapshot` for each cached game that is missing from the list: it finished while the client was away.
  3. `challenge.list` replaces the pending challenges.
  4. `catalog.list` refreshes the catalogue.
- Retry an action that got `stale` only after applying the newer update.
- Send a heartbeat within each `heartbeat_ms`.

## Conformance

Each `conformance/*.ndjson` file is a transcript replayed against a fresh
server with users `alice`, `bob` and `carol`, all with the password
`correct horse`. Each line is one of:

```json
{"comment": "..."}
{"client": "alice", "send": {...envelope...}}
{"client": "alice", "recv": {...envelope...}}
{"client": "alice", "close": true}
```

- A client connects on its first `send`, and again on its first `send` after a `close`.
- `recv` takes the first message that client has received, in arrival order, that matches. This allows pushes and replies to interleave. Objects match on exactly the same keys.
- A string `"$name"` is a placeholder for an id, a time or a UUID. It matches any value the first time and binds `name` to it. Every later use of the same name, in any `recv`, must equal the bound value, and in `send` it is replaced by that value.
- A string starting `$$` is a literal: `"$$x"` matches or sends `"$x"`.
- At the end, and before each `close`, any message the transcript did not expect fails the replay.
- The replay also validates every server line against `schema/`.
- `solo_counter` is a test game the replay server registers, so that a solo start is covered.

The Go replay is `TestConformanceTranscripts` in `server/internal/hub`. It
runs the real hub, store and sessions, with a clock that does not advance
during the run.
