# games.nvim — notes for coding agents

Turn-based games inside Neovim. A Go server (`server/`) owns all state and rules; the Lua plugin (`lua/games/`) renders boards and forwards input over NDJSON/TCP. User docs: [README.md](README.md), [INSTALLATION.md](INSTALLATION.md); wire protocol: [docs/protocol.md](docs/protocol.md). Open work: [TODO.md](TODO.md).

## Invariants
- **Game logic lives in Go; Lua only renders.** A rule, validation or derived value the client needs belongs in the server's view or result. Some chess puzzle-import parsing still sits in Lua; TODO.md tracks moving it.
- **The protocol is free to change.** No external clients yet: change the wire in place, no version bumps, no compatibility shims, then `make schema`.
- **Game rules are deterministic** from the `*rand.Rand` they are given: no wall clock, no globals, no goroutines.
- **Simplest design wins.** No speculative knobs, caches or sync machinery the task does not need.

## Layout
| Path | What |
|---|---|
| `server/game/` | rules framework: `Rules[Opts, Action, View]`, options, seats, turns, endings; shared `grid/`, `cards/`, `dice/`; `gametest/` harness |
| `server/games/<id>/` | one package per game; `games/registry.go` lists them |
| `server/internal/protocol` | wire types, error codes, request catalog |
| `server/internal/session` | one connection: framing, heartbeat, pre-login dispatch (`auth.login`, `auth.register`) |
| `server/internal/hub` | sessions per user, routing, challenges, sign-up, pause-on-disconnect |
| `server/internal/room` | one actor goroutine per live game: steps, clocks, ticks, persistence |
| `server/internal/store` | SQLite (one writer connection, pooled readers, WAL) |
| `server/internal/{auth,archive,limits,clock}` | bcrypt + passphrase + IP lockout; summaries/digests; limits; injectable clock |
| `server/cmd/games-server` | `serve`, `adduser`; `cmd/schemagen` writes `docs/schema/` |
| `lua/games/core/` | `conn` (socket, login, reconnect), `client` (cache, resync), `login` (prompts, register), `notify`, `sources` |
| `lua/games/ui/` | `compose` + `paint` rendering, `theme`/`highlights`, `widgets/`, `shell/` (board buffers), `finder/`, `dashboard` |
| `lua/games/games/<id>.lua` | declarative per-game layout and actions |
| `lua/games/assets/<game>/<pack>.lua` | glyphs and art (swappable asset packs) |
| `docs/schema/`, `docs/conformance/` | generated JSON Schema + catalog; replayable NDJSON transcripts |

## Architecture notes
- **Room:** a ticking game (pacman, metronome) schedules its next tick from the due time (`nextTickAt`), clamped to now after a stall, so there is no catch-up burst. Tick steps are written behind in batches (`tickBatchAge`, 250 ms); every other step, finish, close and shutdown flushes first. A restore replays steps exactly; a crash loses at most the unflushed ticks and restores paused.
- **Disconnect:** when a user's last session closes, the hub pauses their ticking games, but only if they are still away when the pause runs.
- **Store:** the single SQLite writer is a server-wide queue, so one room's slow write delays every room. Moving tick-batch writes off the room goroutine is parked in TODO.md (worth it only for ≤50 ms ticks or many ticking rooms).
- **Client cache:** pushes apply only when newer by `seq` (`StoreMode.NEWER`); resync after a reconnect replaces the cache (`StoreMode.REPLACE`), since a restarted server can be behind seqs it already pushed.
- **Rendering:** grids emit one prerendered block per row with a per-board row cache; marks are parallel arrays; shell updates are coalesced to one render of the latest; the client socket sets `nodelay`. These were measured; keep them unless a benchmark says otherwise.
- **Auth:**
  - login is username + password (bcrypt; `auth.DefaultPasswordCost`); 5 failures in 5 min lock the IP for 5 min;
  - sign-up (`auth.register`, `:Games register`) works only when `serve` has a passphrase (`--signup-passphrase-file` or `GAMES_SIGNUP_PASSPHRASE`);
  - order: lockout → enabled → passphrase (constant-time) → name → password → exists → create;
  - `adduser` and sign-up share `auth.CreateAccount`;
  - user names are ≤32 characters, with no control characters or edge whitespace; this is checked at creation only.

## Commands
| Command | Use |
|---|---|
| `make check` | **the gate**: lint (gofmt, vet, stylua, LuaLS), schema-check, `go test -race -short`, Lua specs |
| `cd server && go test -race -count=1 ./...` | full Go suite; run on every Go change |
| `make schema` | after changing any wire type, option, event or reason; `make schema-check` fails on drift |
| `make smoke-finders` | after touching the finder, `tests/lua/helpers.lua` or `tests/lua/fixtures/` (not in `make check`) |
| `make fmt` | gofmt + stylua; never reformat files you did not touch |
| `make build` / `make docker` | `bin/games-server` / `games-server:latest` |

Gate on exit codes, not piped output: `make check > log 2>&1; echo exit=$?`.

## Tests
- **Go game rules:** `gametest.Run(t, <id>.Rules{}, scripts)` replays scripts and checks views, events, privacy, results and restore; `NewState`, `PlaySolo`, `Resigned`, `SeatsFirstMover` cover the common cases. `-short` only trims chess perft.
- **Go server:** hub tests start a real hub on a test listener (`prototest`, `storetest`) with a fake clock (`clocktest`).
- **Conformance transcripts:** `docs/conformance/*.ndjson` hold one line per send, receive, close or clock advance, with an optional `{"seeds": [...]}` header. They are replayed by `hub.TestConformanceTranscripts` with a 2 s receive deadline. Add or update one when a request's wire behaviour changes.
- **Lua specs:**
  - `tests/lua/*_spec.lua` each return a table of named cases, run headless by `tests/lua/run.lua`;
  - `make test-lua` builds the server first, because `e2e_spec` and `helpers.with_server` spawn a real `bin/games-server` on a free port;
  - stubs: `helpers.stub_conn` (with `conn:sent`, `conn:respond`, `conn:push`), `fixtures/hub.lua`, `helpers.render_game`, `helpers.with_overrides`.
- **Schema ↔ Lua types:** `tests/lua/schema_spec.lua` checks the Lua `---@class` annotations against `docs/schema`; renamed definitions go in its `LUA_TYPE_EXCEPTIONS`.
- **Goldens:** render snapshots (e.g. `chess_golden_spec`, the dashboard) stay byte-identical unless the render is meant to change.
- **Smoke:** `tests/smoke/finders.lua` drives snacks.picker, telescope, fzf-lua and `vim.ui.select` for real (accept + cancel each); it shares `tests/lua` helpers, so renaming a helper there can break it.

### Timing and flake rules
- **bcrypt dominates hub test time.** Test users hash at `bcrypt.MinCost`, and the test hub gets `PasswordCost: bcrypt.MinCost` (`hub.Deps`), so logins and sign-ups fit the 2 s deadline under `-race`. Never hash at the default cost in tests.
- **Don't race the disconnect pause.** A test that closes the last session and logs back in must first `awaitStoredPause` (`hub/tick_test.go`); a fast login otherwise overtakes the pause.
- **Wait on state, not time:** use `prototest.WaitFor` or `clocktest` advances, not sleeps.
- **Reproduce CI timing locally** with `GOMAXPROCS=2 go test -race -count=5 ./internal/hub/`. Fix a flake's cause; don't delete the test or raise the deadline.

## CI/CD
- Workflow: `.github/workflows/ci.yml`, on every push and PR, `ubuntu-latest`, matrix over Neovim `stable` and `v0.11.0` (the oldest supported).
  - `stable`: `make check`, then fzf (pinned version + sha256) and `make smoke-finders`.
  - `v0.11.0`: `make test-lua` only.
- Tools are pinned in the workflow env and installed from release tarballs into `$RUNNER_TEMP`: Neovim, stylua, lua-language-server, fzf. Neovim is not installed through `action-setup-vim`, which fails across filesystems on the Ubuntu 26 runners.
- `actions/setup-go` reads `server/go.mod`, with its cache keyed on `server/go.sum`.
- Runners are slow 2-CPU machines with `-race`: timing that passes locally can fail there (see the flake rules above).
- There is no release or deploy pipeline. The server ships as source (`go install …/server/cmd/games-server@latest`) or the Docker image (`Dockerfile`, `compose.yaml`; distroless, `nonroot`, DB on the `games-data` volume, `GAMES_BIND`/`GAMES_PORT`/`GAMES_SIGNUP_PASSPHRASE` passed through).
- Job logs need a GitHub login; annotations and run status are readable through the public API.

## Conventions
- **Reuse first.** Search before writing a helper; put missing shared code in the shared place, not inside one game.
  - Go: `game` (`OptionValues`, `TimeControlOption`, `FirstMoverOption`/`PickFirst`, `SideChoice`, `EncodeEvent`, `Ending.ToActOf`, `Scorer`), `game/grid` (`Rows`, `WinningLines`, `WinningCells`, `Neighbours`, `Edges`), `game/gametest`, `internal/xslices`, `internal/prototest`.
  - Lua: `ui/text` (width, padding, `widest`, `center_to_width`/`center_to_chars`), `ui/compose`, `ui/paint`, `ui/theme` + `ui/highlights`, `ui/widgets/{grid,kv,movelist}`, `core/players`, `core/describe`, `core/notify` (`say`/`error`/`warn` — the only way to notify).
- **Strong types:** named types for ids, seats, kinds and reasons (`type X string` + consts in Go; `---@alias`/`---@enum` in Lua). Enums over bools for modes.
- **Names over comments.** Comment only a non-obvious why or a complex algorithm. Doc comments on exported Go identifiers and LuaCATS (`---@param`/`---@return`/`---@class`) are expected; LuaLS must be clean.
- **Pure and immutable by default;** lookup tables are module-level and never mutated. One function, one job.
- **Go:** idiomatic, stdlib-first (`slices`, `maps`, `cmp`); wrap errors with `%w`; small interfaces where consumed; view JSON `snake_case`, arrays never `null`. No new dependencies without asking.
- **Lua:** `local M = {} … return M`, no globals, Neovim 0.11+ APIs. Game modules hold no window, buffer, keymap, highlight, glyph or art code. Existing plugins (pickers) are fine behind a thin adapter.
- **Match the file you edit**, even where you would choose differently. Fix defects at the layer that owns them.
- **Tests prove behaviour:** table-driven where cases vary; a test that cannot fail is a defect. When fixing a test, mutation-check it: break the code it guards and see it fail.
- **Adding a game:** see README "Adding a game" (package + registry line + `make schema` + one declarative Lua module; no core edits).

## Security
- Never print, log or notify password or passphrase values, including in tests, logs and commit messages.
- The sign-up passphrase is never a flag value (visible in `ps`).
- Transport is plain TCP; keep the default `127.0.0.1` bind.

## Workflow
- `main` is the default branch. Commit messages: `area: what changed` (`hub:`, `lua:`, `docs:`, `ci:` …), lowercase, one line, with the why when it is not obvious.
- Open work lives in `TODO.md`, one line per item; delete an entry when done. No issue tracker.
- Don't push unless asked.
- Skills worth loading when available: `superpowers:systematic-debugging` for failures and flakes, `superpowers:test-driven-development` for behaviour changes, `simplify` after a non-trivial change, `code-review` before merging.
