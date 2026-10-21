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
| `game.start` | `{game, options}` | `GameUpdate` (seq 0) | Games that seat one player (`players.min == 1`), solo-only or not; the game is played alone. The first update is also pushed. |
| `stats.get` | `{user?}` | `StatsResult` | See [Stats](#stats). |
| `game.verify` | `{summary}` | `{}` | Checks an archived summary; see [Finished games](#finished-games). |

## Pushes

| Type | Body | Sent to |
|---|---|---|
| `challenge.changed` | `Challenge` | Every participant, on each change: created, an accept, accepted, declined, cancelled, expired. |
| `game.update` | `GameUpdate` | Every player, after every step (an action, a control, a timeout, or a real-time game's tick or pause). Each recipient gets its own `view` and `controls`. |

## Error codes

| Code | When |
|---|---|
| `bad_request` | Malformed envelope or body, a request without `id`, unknown type, unknown game, user, option or `status`, an invalid option value, an invalid invitee list (empty, the wrong size for the game, a duplicate, or the challenger), `game.start` of a game that cannot seat one player, an unknown or already resolved challenge, a challenge the user may not resolve, an action the game cannot parse, a second `auth.login`. |
| `version_mismatch` | `auth.login` with another protocol version. |
| `unauthenticated` | Any request but `auth.login` before login. |
| `auth_failed` | Wrong credentials, or the IP is locked out. |
| `not_found` | No such game, the user is not one of its players, or `game.act`/`game.control` on a finished game the server has dropped (see below). |
| `not_your_turn` | `game.act` when the user is not in `to_act`. |
| `illegal_action` | The game rejects the action, the control is not in `controls`, or the game is over but the server still holds it. |
| `stale` | `seq` is not the game's current seq; in a real-time game, ticks and pauses past it do not count (see [Ticks](#ticks)). Nothing changed. |
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
| `controls` | The controls this recipient may send now, from `resign`, `offer_draw`, `decline_draw` and `abort`. `abort` is offered only until every player has acted once. A player out of the game gets none: one who resigned or timed out, or one the rules knocked out (busted at poker, out of dice at liar's dice), who also loses an ending the core imposes. |
| `events` | The events of the step this update reports. Each is an object with a `kind`; see `x-events`. At seq 0 they are the events of the setup, such as moves the rules play before anyone acts. Snapshots and later reads have `[]`. |
| `view` | The game's view as this recipient sees it; see `x-view`. A finished game keeps each player's final view. |
| `started_at` | Unix ms when the game started. |
| `finished_at`, `summary`, `digest`, `exports` | `null` until finished; see [Finished games](#finished-games). |

Arrays are never `null`. Every field is always present.

### Seq rules

- `seq` counts the game's steps: 0 at the start, plus 1 for each accepted action, control, timeout, tick or pause.
- `game.act` and `game.control` carry the seq the client acted on. If it differs from the current seq, the reply is `stale` and nothing changes. A real-time game's ticks and pauses are the exception: see [Ticks](#ticks). The reply's `seq` is the step's new seq, and the matching `game.update` push carries the same seq.
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

### Ticks

A real-time game, such as Pacman, moves on its own. The server steps it in
ticks; the game sets the time between them, which can change as it goes, and
stops them while it waits on its player.

- A tick is a step like any other: it takes the next seq, is stored, replayed on a restart, and pushed as a `game.update` with the events it made. A game ticks at several steps a second, so expect that many pushes; each is the whole game.
- Input does not delay a tick: an action keeps the next tick at the time it was due.
- A seq is stale only once a player's step (an action or a control) came after it. Ticks and pauses after it do not make it stale, so a client may act on the latest update it has even if ticks went past it in flight. The reply's `seq` is still the step's new seq.
- **Pause.** The server pauses a real-time game when its player's last session closes, and when it restarts with the game ongoing; a pause is a step of its own. The game may also take a pause action from its player. While paused, no ticks come. The game's own actions resume it; Pacman resumes on any turn.
- No tick comes once the game is finished.

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

### Tic-Tac-Toe

- Reasons: `three_in_a_row`, `board_full`.
- Events: `placed {by: seat, cell}`, `cell` an integer 1-9.
- Seats: `x`, `o`; `x` moves first. Option `first` picks who plays `x`.
- View `{cells, last, winning_lines, legal_cells}`:
  - cells are numbered 1-9 in phone layout: 1 top-left, 3 top-right, 9 bottom-right;
  - `cells` is always 9 strings, each `x`, `o`, or `""` when empty, in cell order;
  - `last` is the cell placed last, or `null` before the first move;
  - `winning_lines` lists each completed line as its cells, `[]` unless won; one move can complete two;
  - `legal_cells` is the viewer's own: the empty cells when the viewer is to move, else `[]`.
- Actions: `place` (`cell`: the cell's number as text, `"1"` to `"9"`).

### Connect4

- Reasons: `four_in_a_row`, `board_full`.
- Events: `dropped {by: seat, row, column}`, the cell the disc came to rest in.
- Seats: `red`, which moves first, and `yellow`.
- View `{cells, last: {row, column} | null, winning_lines, legal_columns}`:
  - `cells` is 6 rows of 7, row 1 at the top; each is the seat whose disc fills it, or `""`.
  - `winning_lines` is `[]` until a win, then every run of 4 or more through the last disc, whole.
  - `legal_columns` holds the viewer's own: the open columns, empty unless the viewer is to move.
- Actions: `drop` (`column`: `"1"` to `"7"`).

### Wordle

- Solo: started with `game.start`; the only seat is `solver`.
- Reasons: `solved`, `out_of_guesses`.
- Events: `guessed {by: seat, word, feedback}`.
- View `{guesses, keyboard, remaining, answer?}`:
  - `guesses` lists each valid guess, oldest first, as `{word, feedback}`; `[]` at the start;
  - `feedback` is always 5 marks, one per letter: `hit` (right place), `present` (elsewhere in the word) or `miss`;
  - `keyboard` maps each guessed letter to its best mark so far (`hit` over `present` over `miss`); letters not yet guessed are absent;
  - `remaining` is the number of guesses left, 6 at the start;
  - `answer` is absent until the game finishes, however it ends.
- Actions: `guess` (`text`: five letters, any case; surrounding spaces are ignored). A word outside the list is an `illegal_action` and costs no guess.

### Sudoku

- Solo: one seat, `solver`; start it with `game.start`. Option `difficulty`: `easy`, `medium` (default), `hard`.
- Reasons: `solved`.
- Events: `cell_set {by: seat, row, col, digit}`, `row` and `col` 1-9 from the top left, `digit` 0 when cleared.
- View `{grid, given, conflicts, solution}`:
  - `grid` is always 81 integers in row-major order, cell `(row-1)*9 + col`, 0 when empty;
  - `given` is 81 booleans, the puzzle's own cells;
  - `conflicts` is 81 booleans, the filled cells whose digit repeats in their row, column or box;
  - `solution` is `[]` until the game is over, then all 81 digits.
- Actions: `set` (`text`: `"RC D"`, row, column and digit, e.g. `"53 7"`; `"53 0"` clears). The catalogue gives the pattern.

### Gomoku

- Reasons: `five_in_a_row`, `board_full`.
- Events: `placed {by: seat, cell: {row, column}}`.
- Seats: `black`, which moves first, and `white`. Option `first` gives black to the challenger (`me`), the opponent, or either at random.
- View `{cells, last: {row, column} | null, winning_lines, legal_cells}`:
  - `cells` is 15 rows of 15, row 1 at the top; each is the seat whose stone fills it, or `""`.
  - `winning_lines` is `[]` until a win, then every run of 5 or more through the last stone, whole.
  - `legal_cells` holds the viewer's own: the empty cells as `{row, column}`, empty unless the viewer is to move.
- Actions: `place` (`cell`: a column letter `a`-`o` then a row number `1`-`15`, e.g. `"h8"`; either case, surrounding spaces ignored). The catalogue gives the pattern.

### Reversi

- Reasons: `more_discs` (decisive), `equal_discs` (draw); both mean neither seat could move.
- Events:
  - `placed {by: seat, cell: {row, column}, flipped: [{row, column}]}`, every disc the move turned over;
  - `passed {by: seat}`: the seat had no legal move, so its opponent moves again. The server passes for it; there is no pass action. It follows the `placed` that blocked the seat, in the same update.
- Seats: `black`, which moves first, and `white`; option `first` as in Gomoku.
- View `{cells, last: {row, column} | null, legal_cells, discs: {black, white}}`:
  - `cells` is 8 rows of 8, row 1 at the top; it starts with white on d4 and e5, black on e4 and d5.
  - `legal_cells` holds the viewer's own: the cells where its disc would flip at least one, empty unless the viewer is to move.
  - `discs` counts each seat's discs on the board.
- Actions: `place` (`cell`: `a`-`h` then `1`-`8`, e.g. `"d3"`), as in Gomoku. A cell that flips nothing is an `illegal_action`.

### Chess puzzle

- Solo: one seat, `solver`; start it with `game.start`. The client supplies the whole line in the options:
  - `mode`: `puzzle` (default; a wrong move ends the game) or `study` (a wrong guess is a miss);
  - `fen`: the start position, text of at most 100 bytes; `""` is the standard start;
  - `moves`: the line, both sides, space-separated SAN or UCI, 1 to 600 plies;
  - `player`: `white` (default) or `black`, the side the solver plays.
- `game.start` checks the line: a FEN that does not parse, a move that does not replay (`ply 3: illegal move: Nf6`), a line of 0 or more than 600 plies, or one with no move for `player` is a `bad_request`.
- The server plays every move of the other side, at the start and after each of the solver's, so a Lichess row's first move (the opponent's) needs no special case.
- Reasons: `solved` (puzzle: the line played out, or any mate), `wrong_move` (puzzle, a loss), `completed` (study, a win).
- Events, all `by` the solver's seat: `moved {by, san}` (the solver's accepted move), `replied {by, san}` (the server's move for the other side), `missed {by, san}` (a legal move that is not the line's), `revealed {by, san}` (the line's move, played on `reveal`).
- View `{fen, history_san, last_move, legal_moves, check_square, mode, misses, remaining, solution}`:
  - `fen`, `history_san`, `last_move` and `check_square` are as in chess; `history_san` starts at the given FEN;
  - `legal_moves` is in UCI while the game runs, `[]` once it is over, however it ends;
  - `misses` counts missed and revealed moves; `remaining` counts the solver's moves left in the line;
  - `solution` is `[]` until the game is over, then the whole line in SAN.
- Actions: `move` (`text`: SAN or UCI, any case) and `reveal` (study mode only). A move that is not legal is an `illegal_action` and costs nothing. In puzzle mode a move other than the line's that mates is accepted as the solve.

### Mastermind

- Solo: started with `game.start`; the only seat is `solver`.
- Reasons: `solved`, `out_of_guesses`.
- Events: `guessed {by: seat, code, black, white}`.
- View `{palette, code_length, guesses, remaining, secret?}`:
  - `palette` is the 6 colour letters, `["r", "g", "b", "y", "o", "p"]`; `code_length` is 4;
  - `guesses` lists each guess, oldest first, as `{code, black, white}`; `[]` at the start. `black` counts pegs of the right colour in the right place, `white` the right colour in the wrong place;
  - `remaining` is the number of guesses left, 10 at the start;
  - `secret` is absent until the game finishes, however it ends.
- Actions: `guess` (`code`: 4 palette letters, any case, e.g. `"rgby"`; surrounding spaces are ignored). The catalogue gives the pattern.

### Hangman

- Solo: started with `game.start`; the only seat is `solver`.
- Reasons: `solved`, `hanged`.
- Events: `guessed {by: seat, guess, hit}`: `guess` is a letter or a whole word, `hit` whether the word holds the letter, or is that word.
- View `{pattern, guessed, missed_words, misses, remaining, word?}`:
  - `pattern` is one string per letter of the word (5): the letter once found, `""` while hidden; every letter once solved;
  - `guessed` lists each letter guessed, oldest first, as `{letter, hit}`;
  - `missed_words` lists each wrong whole-word guess, oldest first;
  - `misses` counts wrong letters and wrong words; `remaining` is the misses left, 6 at the start;
  - `word` is absent until the game finishes, however it ends.
- Actions: `guess` (`text`: one letter, or a 5-letter word from the word list; any case, surrounding spaces ignored). A repeated guess, or a word outside the list, is an `illegal_action` and costs nothing. The catalogue gives the pattern.

### Minesweeper

- Solo: one seat, `solver`; start it with `game.start`. Option `difficulty`: `easy` (default, 9x9 with 10 mines), `medium` (16x16, 40), `hard` (16 rows by 30 columns, 99).
- Reasons: `mine` (a loss, no winner), `cleared`.
- Events, `row` and `col` 1-based from the top left:
  - `revealed {by: seat, row, col, opened}`, `opened` the cells it opened, 0 for a mine;
  - `flagged {by: seat, row, col, flagged}`, `flagged` false when the flag came off;
  - `chorded {by: seat, row, col, opened}`, on the number chorded.
- View `{cells, flags, remaining, chordable}`:
  - `cells` is rows of strings, row 1 at the top: `hidden`, `flag`, or a revealed count `"0"` to `"8"`; once the game is over every unflagged mine is `mine`, a flagged one `flagged_mine`, the one revealed `exploded`, and a flag on a safe cell `wrong_flag`;
  - no cell is `mine`, `flagged_mine`, `exploded` or `wrong_flag` before the end, and nothing else in the view or events locates a mine;
  - `remaining` is the mine count less `flags`, negative with more flags than mines;
  - `chordable` lists `{row, col}` of the revealed numbers a chord acts on now; `[]` once over.
- Actions `reveal`, `flag` and `chord`, each with `cell`: a row letter `a`-`p`, either case, then a column number `1`-`30`, e.g. `"c12"`. The catalogue gives the pattern; a cell off the chosen board is an `illegal_action`.
- The mines are placed by the first `reveal`, from that step's seeded random source, never on the revealed cell or its neighbours.

### 15-puzzle

- Solo: one seat, `solver`; type `fifteen`, start it with `game.start`. Option `size`: `3x3` or `4x4` (default).
- Reasons: `solved`.
- Events: `slid {by: seat, tile, direction}`, `direction` the way the tile moved.
- View `{tiles, moves, movable}`:
  - `tiles` is rows of integers, row 1 at the top, `0` for the blank;
  - `moves` counts the slides so far, the score;
  - `movable` lists `{tile, direction}` for each tile next to the blank and the way it would move; `[]` once over.
- Actions: `slide` (`tile`: the number as text, `"1"` to `"15"`; one the board lacks is an `illegal_action`) and `move` (`direction`: `up`, `down`, `left` or `right`, the way the tile moves, so `up` moves the tile below the blank).
- The shuffle is drawn at `game.start` from the seed, always solvable and never already solved.

### 2048

- Solo: one seat, `solver`; start it with `game.start`. No options.
- Reasons: `no_moves`. It is a win (`winners: ["solver"]`) if a 2048 tile was ever made, else a loss (`winners: []`). Giving up is always a loss.
- Events:
  - `moved {by: seat, direction, gained, spawned: {row, col, value}}`: `gained` is the sum of the tiles the move merged; `spawned` is the 2 or 4 that appeared after it, `row` and `col` 1-4 from the top left;
  - `reached {by: seat, tile: 2048}`, once, after the `moved` of the move that first made a 2048.
- View `{cells, score, reached, movable}`:
  - `cells` is 4 rows of 4, row 1 at the top; each is a tile's value, or 0 when empty;
  - `score` is the sum of every merge so far;
  - `reached` is true once a 2048 has been made; play goes on;
  - `movable` lists the directions that would change the board, in the order `up`, `down`, `left`, `right`; `[]` once the game is over.
- Actions: `move` (`direction`: `"up"`, `"down"`, `"left"` or `"right"`). A move that changes nothing is an `illegal_action`. The spawned tile comes from the step's seeded rng, so a restore replays it.

### Mancala

- Kalah(6,4). Reasons: `more_seeds` (decisive), `equal_seeds` (draw); both mean one side's pits were empty.
- Events:
  - `sowed {by: seat, pit, captured, again}`: `pit` is 1-6 in the mover's numbering; `captured` counts the seeds a capture took into its store, 0 for none; `again` is true when the mover moves again;
  - `collected {by: seat, seeds}`: at the end, the seeds left in that seat's pits went to its store. It follows the `sowed` that ended the game, once per seat with seeds left.
- Seats: `south`, which moves first, and `north`; option `first` as in Connect4.
- View `{south: {pits, store}, north: {pits, store}, legal_pits}`:
  - each side's `pits` is 6 counts in that seat's own numbering: `pits[0]` is its pit 1, on its left, and `pits[5]` its pit 6, next to its store;
  - sowing runs counter-clockwise: south's pits 1-6, south's store, north's pits 1-6, north's store, skipping the opponent's store;
  - `legal_pits` holds the viewer's own: its pits with seeds, empty unless the viewer is to move.
- Actions: `sow` (`pit`: `"1"` to `"6"`, the mover's own pit). An empty pit is an `illegal_action`.

### Yahtzee

- Seats `p1`-`p4` in invite order, `p1` first; one player starts it solo with `game.start`.
- Reasons: `scorecards_full`. A tie of every seat is a `draw`; a tie at the top otherwise, including every seat still playing after one left, gives several winners.
- Events: `rolled {by: seat, dice}`, all five faces after the roll; `scored {by: seat, box, score, yahtzee_bonus}`, `yahtzee_bonus` 0 or 100. `hold` sends no event.
- View `{round, dice, held, rolls_left, cards, can_roll, can_hold, options}`:
  - `dice` is `[]` before the turn's first roll, then 5 faces 1-6; `held` is always 5 booleans;
  - `cards` is every seat's `{seat, scores, upper_total, upper_bonus, yahtzee_bonus, total, out}` in seat order; `scores` maps each filled box to its points;
  - `can_roll` (false with all five held), `can_hold` and `options` are the viewer's own: `options` lists `{box, score}` for every box it may fill now, under the forced joker rule, and is `[]` unless the viewer is to act.
- Actions: `roll`, `hold` (`die`: `"1"` to `"5"`, toggles), `score` (`box`: one of the 13 box names). The catalogue gives the values.
- Scores are the core's: +1 a win, −1 a loss. Card totals are in the view.

### Liar's Dice

- Seats `p1`-`p6` in invite order; `p1` opens round 1, the loser of a die opens the next.
- Reasons: `last_with_dice`.
- Events: `bid {by: seat, quantity, face}`; `challenged {by: seat, bid, counted, loser, cups: [{seat, dice}]}`, every cup as shown; `out {seat}`, a seat lost its last die. A `challenged` and any `out` share the update that rerolls the cups.
- View `{round, seats, bid, total_dice, my_dice, min_bids, can_dudo, last_reveal}`:
  - `seats` is every seat's `{seat, dice, out}`, `dice` the count it holds;
  - `bid` is the standing `{by, quantity, face}`, or `null` before the round's first bid;
  - `my_dice` holds the viewer's own dice only, sorted; no other seat's dice reach a view or event before a `challenged`;
  - `min_bids` is the viewer's own `[{face, quantity}]`, the least legal quantity per face, `[]` unless it is to act; `can_dudo` likewise;
  - `last_reveal` is the last challenge `{round, bid, challenger, counted, loser, cups}`, or `null`.
- Actions: `bid` (`bid`: `"QxF"`, e.g. `"3x5"`; the catalogue gives the pattern) and `dudo`. Ones are wild; the raise rules are in `docs/games/liars_dice.md`.

### Uno

- Seats `p1`-`p6` in invite order, clockwise; a random dealer, whose turned-up card acts as if they had played it.
- Reasons: `went_out`.
- Cards are colour then face: `R`, `Y`, `G`, `B` and `0`-`9`, `S` (skip), `R` (reverse), `+2`; wilds `W` and `W+4`.
- Events: `started {by, dealer, top}`; `played {by, card, color}`, `color` the colour in play after it; `drew {by, seat, count}`, the cards drawn reaching only `seat`'s view; `passed {by}`, keeping the card drawn; `skipped {by, seat}`; `reversed {by, direction}`; `color {by, color}`, naming the colour for a Wild turned up; `challenged {by, seat, guilty}`; `reshuffled {by, cards}`, a new draw pile of `cards`; `left {by}`.
- View `{top, color, direction, phase, draw_pile, seats, my_hand, legal, playable}`:
  - `color` is `""` only while a Wild turned up waits to be named; `direction` is `clockwise` or `counterclockwise`;
  - `phase` is `color`, `play`, `drawn` (play the card drawn, if it matches, or keep it; every draw that takes a card enters it) or `challenge` (challenge the Wild Draw Four or accept it);
  - `seats` is every seat's `{seat, cards, out}`, `cards` the count it holds;
  - `my_hand` holds the viewer's own cards only; no other seat's cards, nor the draw pile's, reach a view or event;
  - `legal` (action kinds) and `playable` (cards) are the viewer's own, `[]` unless it is to act.
- Actions: `play` (`card`: e.g. `"r5"`, or a wild and the colour it names, `"w g"`; the catalogue gives the pattern), `draw`, `pass`, `challenge`, `accept`, `color` (`color`: `R`, `Y`, `G` or `B`). The rulings are in `docs/games/uno.md`.

### Hearts

- Seats `p1`-`p4` in invite order, clockwise; exactly four players. Hands are played until a score reaches 100.
- Reasons: `score_limit`. All four tied at the end is a draw.
- Events: `hand_started {by, hand_no, pass}`, hand 1's in the game's first update; `pass_chosen {by, seat}`, naming no cards; `passed {by, pass}`, the cards changing hands; `played {by, seat, card}`; `trick_won {by, seat, points}`; `hand_scored {by, score: {hand_no, points: [{seat, points}], moon}}`; `left {by}`. `seat` differs from `by` for a seat that left and is played for.
- View `{hand_no, pass, phase, score_limit, seats, trick, last_trick, hearts_broken, my_hand, legal, must_pass, pass_to, passed, received, last_hand}`:
  - `pass` is `left`, `right`, `across` or `hold`; `phase` is `pass` or `play`;
  - `seats` is every seat's `{seat, score, taken, pass_chosen, out}`;
  - `trick` is `[{seat, card}]` in play order; `last_trick` is `{plays, winner, points}` or `null`; `last_hand` is a `hand_scored` score or `null`;
  - `my_hand`, `legal`, `must_pass`, `pass_to`, `passed` and `received` are the viewer's own; the cards a seat passes reach only its view and its receiver's;
  - `legal` is `[]` unless the viewer is to play, `must_pass` 0 once it has chosen, `pass_to` `null` in a hand without passing.
- Actions: `pass` (`cards`: three cards apart, e.g. `"2c qs ah"`) and `play` (`card`: e.g. `"qs"`); the catalogue gives the patterns. The rules are in `docs/games/hearts.md`.

### Battleship

- Seats `p1` and `p2`; `p1` fires first, and the `first` option picks who is `p1`. Both seats place at once: `to_act` holds every seat whose fleet is incomplete, then the seat to fire.
- Squares are `{row, col}`, 1-based from the top left; actions name them as text, the column's letter then the row's number (`"c5"`), with the catalogue's pattern.
- Reasons: `fleet_sunk`.
- Events: `ready {by}`, a fleet complete; `shot {by, cell, outcome, ship?}`, `outcome` one of `miss`, `hit`, `sunk`, with `ship` naming the ship sunk. A placement that leaves a fleet incomplete has no event.
- View `{phase, own, target, fleet, enemy, next, anchors, targets, last}`:
  - `phase` is `setup`, `battle` or `over`;
  - `own` and `target` are rows of tiles, `empty`, `ship`, `miss`, `hit` or `sunk`: `own` is the viewer's waters with the opponent's shots, `target` the opponent's as the viewer's shots found them. No square of the opponent's fleet reaches a view, event or error before it is hit; once the game is over `target` shows its unhit ships as `ship`;
  - `fleet` and `enemy` are `[{name, size, state}]` in placing order, `state` one of `unplaced`, `afloat`, `sunk`; the opponent's ships are only `afloat` or `sunk`;
  - `next` is the viewer's ship to place, `{name, size}`, else `null`; `anchors` is `{across, down}`, the squares its left or top end may go on, empty unless placing;
  - `targets` is the squares the viewer may fire at, `[]` unless it is to fire;
  - `last` is the latest shot `{by, cell, outcome, ship?}`, or `null`.
- Actions: `place_across` and `place_down` (`cell`) place the next ship from that square rightward or downward; `random` places the rest of the fleet from the server's seeded rng; `fire` (`cell`). The fleet and its order are in `docs/games/battleship.md`.

### Chess960

- Chess with a drawn start position; everything not listed here is as in chess: options, seats, reasons, the `moved` event, actions and `game.pgn`.
- The start position is drawn from the game's seed at the start: one of 960 back ranks (bishops on opposite colours, the king between the rooks), mirrored for black.
- View: chess's, plus `start_position` (its number, 0 to 959, in Scharnagl's numbering; 518 is the standard array) and `start_fen` (the start position's FEN, which the move list numbers from).
- Castling ends as in chess, the king on the g- or c-file and the rook on the f- or d-file. It is written as the king's move onto its own rook: UCI `f1h1`, also in `legal_moves` and `last_move`. `move` also takes `O-O` and `O-O-O`; a king's move to the g- or c-file is a plain king move, never a castle.
- The FEN castling field is X-FEN: `K`, `Q`, `k` or `q` while the castling rook is the outermost rook of its colour on that side of the king, otherwise that rook's file (`G` for a white rook on g1 with another on h1, lower case for black). The server reads only standard FENs.
- `game.pgn` adds `[Variant "Chess960"]`, `[SetUp "1"]` and the start `[FEN]`, even from position 518.

### Dots and Boxes

- Seats `p1` and `p2`; `p1` moves first, and the `first` option picks who is `p1`. Option `size`: `3`, `5` or `7` boxes a side, default `5`.
- Edges are `{row, col, orientation}`: the dot at the edge's left end lying `across`, or its top end lying `down`, 1-based from the top left on the lattice of dots, which is one row and one column larger than the boxes. Actions name an edge as text, the dot's column letter and row number then `h` (across) or `v` (down), e.g. `"b2h"`, either case, with the catalogue's pattern; the pattern is the 7-box board's, and an edge off a smaller board is an `illegal_action`.
- Reasons: `more_boxes`; `equal_boxes` (a draw) cannot happen at the offered sizes, whose box counts are odd.
- Events: `drawn {by, edge, boxes, again}`, `boxes` the `{row, col}` boxes the edge completed, `[]` for none; `again` is true when `by` moves again, having completed a box with the game still on.
- View `{across, down, boxes, scores, last, legal_edges}`:
  - `across` is `size + 1` rows of `size`, `down` `size` rows of `size + 1`: the seat that drew each edge from that dot, `""` while undrawn; `boxes` is `size` rows of `size`, the seat that completed each box or `""`;
  - `scores` maps each seat to its boxes; `last` is the latest edge drawn, or `null`;
  - `legal_edges` is the viewer's own: every undrawn edge, `[]` unless it is to move.
- Actions: `draw` (`edge`).

## Stats

`stats.get {user?}` → `{records, head_to_head, leaderboard}`, already ranked
by the server.

- A rank orders most wins, then fewest losses, then most draws, then user name.
- `records`: `[{user, game, wins, losses, draws}]`, grouped by game, ranked within each game.
- `head_to_head`: `[{user, opponent, game, wins, losses, draws}]`, by user, opponent, game.
- `leaderboard`: `[{user, wins, losses, draws}]`, totals over every game but those played alone, ranked; a game played alone counts only in its own `records`.
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
server with users `alice`, `bob`, `carol` and `dave`, all with the password
`correct horse`. Each line is one of:

```json
{"comment": "..."}
{"client": "alice", "send": {...envelope...}}
{"client": "alice", "recv": {...envelope...}}
{"client": "alice", "close": true}
{"advance_ms": 120}
```

- A client connects on its first `send`, and again on its first `send` after a `close`.
- `recv` takes the first message that client has received, in arrival order, that matches. This allows pushes and replies to interleave. Objects match on exactly the same keys.
- A string `"$name"` is a placeholder for an id, a time or a UUID. It matches any value the first time and binds `name` to it. Every later use of the same name, in any `recv`, must equal the bound value, and in `send` it is replaced by that value.
- A string starting `$$` is a literal: `"$$x"` matches or sends `"$x"`.
- `advance_ms` moves the server's clock on, firing the timers that come due, such as a real-time game's tick or a flag falling.
- At the end, and before each `close`, any message the transcript did not expect fails the replay.
- The replay also validates every server line against `schema/`.
- `solo_counter` is a test game the replay server registers, so that a solo start is covered.

The Go replay is `TestConformanceTranscripts` in `server/internal/hub`. It
runs the real hub, store and sessions, with a clock that advances only on
`advance_ms`.
