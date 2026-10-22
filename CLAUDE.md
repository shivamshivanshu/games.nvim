# games.nvim — notes for coding agents

Turn-based games inside Neovim. A Go server (`server/`) owns all state and rules; the Lua plugin (`lua/games/`) renders boards and forwards input over NDJSON/TCP. User docs: [README.md](README.md), [INSTALLATION.md](INSTALLATION.md); wire protocol: [docs/protocol.md](docs/protocol.md).

## Invariants
- **Game logic lives in Go; Lua only renders.** A rule, validation or derived value needed by the client belongs in the server's view or result, not in Lua. Some puzzle-import parsing still sits in Lua; see TODO.md.
- **The protocol is free to change.** There are no external clients yet, so change the wire in place: no version bumps, no compatibility shims. Then regenerate the schema (below).
- **Game rules are deterministic** from the `*rand.Rand` they are given: no wall clock, no globals, no goroutines.
- **Simplest design wins.** No speculative knobs, caches or sync machinery the task does not need.

## Layout
| Path | What |
|---|---|
| `server/game/` | rules framework: `Rules[Opts, Action, View]`, options, seats, turns, endings; `grid/`, `cards/`, `dice/` shared game helpers; `gametest/` test harness |
| `server/games/<id>/` | one package per game; `games/registry.go` lists them |
| `server/internal/` | `protocol` (wire types, catalog), `session` (connection, login), `hub` (routing, sessions, sign-up), `room` (one actor per live game), `store` (SQLite), `auth`, `archive`, `limits` |
| `server/cmd/games-server` | `serve`, `adduser`; `cmd/schemagen` writes `docs/schema/` |
| `lua/games/core/` | connection, client cache, login, notify, sources |
| `lua/games/ui/` | compose/paint rendering, theme, highlights, widgets, shell (board buffers), finder, dashboard |
| `lua/games/games/<id>.lua` | declarative per-game layout and actions |
| `lua/games/assets/<game>/<pack>.lua` | glyphs and art (asset packs) |
| `tests/lua/` | headless specs (`run.lua`, `helpers.lua`, `fixtures/`); `tests/smoke/` real-picker smoke |
| `docs/schema/`, `docs/conformance/` | generated JSON Schema; replayable NDJSON transcripts |

## Commands
| Command | Use |
|---|---|
| `make check` | the gate: lint (gofmt, vet, stylua, LuaLS), schema-check, `go test -race -short`, Lua specs |
| `cd server && go test -race -count=1 ./...` | full Go suite, also on every Go change |
| `make schema` | after changing any wire type, option, event or reason |
| `make smoke-finders` | after touching the finder, `tests/lua/helpers.lua` or `tests/lua/fixtures/`; **not** part of `make check`, only CI runs it (needs `fzf`, downloads pickers) |
| `make fmt` | gofmt + stylua (only on files you touched) |

Gate on exit codes, not on piped output: `make check > log 2>&1; echo exit=$?`.

CI (`.github/workflows/ci.yml`): Neovim `stable` runs `make check` + smoke; `v0.11.0` runs `make test-lua` only (oldest supported). Runners are slow 2-CPU machines with `-race`, so timing assumptions that pass locally can fail there; reproduce with `GOMAXPROCS=2 go test -race`.

## Conventions
- **Reuse first.** Search before writing a helper; add missing shared code in the shared place, not inside one game.
  - Go: `game` (`OptionValues`, `TimeControlOption`, `FirstMoverOption`/`PickFirst`, `SideChoice`, `EncodeEvent`, `Ending.ToActOf`, `Scorer`), `game/grid` (`Rows`, `WinningLines`, `WinningCells`, `Neighbours`, `Edges`), `game/gametest` (`Run`, `NewState`, `PlaySolo`, `Resigned`, `SeatsFirstMover`, `Private`), `internal/xslices`, `internal/prototest`.
  - Lua: `ui/text` (width, padding, `widest`, `center_to_width`/`center_to_chars`), `ui/compose`, `ui/paint`, `ui/theme` + `ui/highlights`, `ui/widgets/{grid,kv,movelist}`, `core/players`, `core/describe`, `core/notify` (`say`/`error`/`warn` — the only way to notify).
- **Strong types:** named types for ids, seats, kinds, reasons (`type X string` + consts in Go; `---@alias`/`---@enum` in Lua). Enums over bools for modes.
- **Names over comments.** Comment only a non-obvious why or a complex algorithm. Doc comments on exported Go identifiers and LuaCATS (`---@param`/`---@return`/`---@class`) are expected; LuaLS must be clean.
- **Pure and immutable by default;** lookup tables are module-level and never mutated. One function, one job.
- **Go:** idiomatic and stdlib-first (`slices`, `maps`, `cmp`); wrap errors with `%w`; small interfaces where consumed; view JSON fields `snake_case`, arrays never `null`. No new dependencies without asking.
- **Lua:** `local M = {} … return M`, no globals, Neovim 0.11+ APIs. Game modules have no window, buffer, keymap, highlight, glyph or art code.
- **Match the file you edit**, even where you would choose differently. Fix defects at the layer that owns them.
- **Tests prove behaviour:** table-driven where cases vary; a test that cannot fail is a defect; use `tests/lua/helpers.lua`, `gametest`, the hub fixtures.

## Testing gotchas
- **bcrypt dominates hub test time.** Test users are hashed at `bcrypt.MinCost`, and the hub under test gets `PasswordCost: bcrypt.MinCost` (`hub.Deps`), so logins and sign-ups stay inside the conformance receive deadline (2 s). Never hash at the default cost in tests.
- **Disconnect → reconnect races.** The hub pauses a ticking game only if the user is still away when the pause runs. A test that closes the last session and logs back in must first wait for the stored pause (`awaitStoredPause` in `hub/tick_test.go`), or a fast login overtakes it.
- **Schema and Lua types move together.** After `make schema`, update the Lua `---@class` annotations; `tests/lua/schema_spec.lua` checks them.
- **Conformance transcripts** (`docs/conformance/*.ndjson`) replay real sessions; add or update one when a request's wire behaviour changes.
- **Golden snapshots** in Lua specs must stay byte-identical unless the render is meant to change.

## Security
- Never print, log or notify password or passphrase values (tests, logs, commit messages included).
- The sign-up passphrase comes from `--signup-passphrase-file` or `GAMES_SIGNUP_PASSPHRASE`, never a flag value (visible in `ps`).
- Transport is plain TCP; don't weaken the default `127.0.0.1` bind.

## Workflow
- Open work lives in `TODO.md`, one line per item; delete an entry when it is done. No issue tracker.
- `main` is the default branch. Commit messages: `area: what changed` (e.g. `hub: …`, `lua: …`, `docs: …`), lowercase, one line, the why when it is not obvious.
- Don't push unless asked.
- Skills worth loading when available: `superpowers:systematic-debugging` for failures and flakes, `superpowers:test-driven-development` for behaviour changes, `simplify` after a non-trivial change, `code-review` before merging.
