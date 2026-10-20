# games.nvim

Turn-based games against friends inside Neovim: a Go game server plus a Neovim Lua plugin that renders boards and talks to it over TCP.

Only chess exists today. The server owns all state and rules; Neovim renders what it is sent and forwards your input. Wire format is NDJSON over TCP.

## Requirements

| Part | Needs |
|---|---|
| Server | Go 1.26+ (`server/go.mod`) |
| Plugin | Neovim 0.11+ (uses `vim.uv`, `vim.iter`, and the `vim.validate(name, value, validator, msg)` form; there is no runtime version check) |
| Dev | `stylua`, `lua-language-server` (`LUALS`, default mason path) |

## Server

```sh
make build                      # -> bin/games-server
bin/games-server adduser alice  # prompts for a password
bin/games-server adduser bob
bin/games-server serve
```

There is no self-registration: every account is created with `adduser`.

`serve` flags:

| Flag | Default | Meaning |
|---|---|---|
| `--addr` | `127.0.0.1:7777` | listen address |
| `--db` | `games.db` | SQLite file |
| `--heartbeat` | `15s` | heartbeat interval |
| `--idle` | `45s` | idle timeout before a client is dropped |

`adduser [--db games.db] [--password-stdin] <name>`: `--password-stdin` reads the password from one line of stdin instead of prompting. Use the same `--db` as `serve`.

Failed logins are locked out per IP: 5 failures in 5 min locks that IP out for 5 min.

## Install

lazy.nvim:

```lua
{
  "shivamshivanshu/games.nvim",
  cmd = "Games",
  opts = {
    server = { host = "games.example.net", port = 7777 },
    username = "alice",
    password = function()
      return os.getenv("NVIM_GAMES_PASSWORD")
    end,
  },
}
```

`username` and `password` are optional: if missing, the first `:Games` prompts for them (username via `vim.ui.input`, password hidden via `inputsecret`) and keeps them for the session. `password` may be a string or a function; the function is called on each connect, never during `setup()`, so the secret need not live in your dotfiles.

Other managers all end in `require("games").setup({ ... })`:

```lua
-- packer.nvim
use({ "shivamshivanshu/games.nvim", config = function() require("games").setup({ ... }) end })

-- mini.deps
MiniDeps.add("shivamshivanshu/games.nvim")
require("games").setup({ ... })
```

```vim
" vim-plug
Plug 'shivamshivanshu/games.nvim'
```

Native packages (then `:helptags ALL`, and call `setup()` from your config):

```sh
git clone https://github.com/shivamshivanshu/games.nvim \
  ~/.local/share/nvim/site/pack/games/start/games.nvim
```

Help: `:help games`. Diagnostics: `:checkhealth games`.

## Join a server

1. Ask the host for an account and the server address.
2. Set `server.host` / `server.port` (see Config).
3. Run `:Games`.

## Host a server

```sh
go install github.com/shivamshivanshu/games.nvim/server/cmd/games-server@latest  # or: make build
games-server adduser alice                        # prompts for a password
games-server serve --addr 100.64.0.1:7777         # e.g. your Tailscale address
```

Details and flags are under Server below.

## Config

| Option | Type | Default | Notes |
|---|---|---|---|
| `server.host` | string | `"127.0.0.1"` | |
| `server.port` | integer | `7777` | 1-65535 |
| `username` | string | prompted | optional; prompted on first `:Games` if missing |
| `password` | string or `fun(): string` | prompted | optional; resolved at each connect |
| `autoconnect` | boolean | `true` | connect after `VimEnter` (or next tick if already started); skipped without a `username` |
| `glyphs` | `"unicode"` or `"ascii"` | `"unicode"` | piece glyphs; colours are the same either way |
| `palette` | `"theme"` or `"vivid"` | `"theme"` | colours derived from your colourscheme, or a fixed lichess-style board with near-black ink pieces; `vivid` needs `termguicolors` and falls back to `theme` without it |
| `highlights` | table | `{}` | `{ GamesX = {...} }`, each passed to `nvim_set_hl` in place of the palette's definition |
| `archive.dir` | string | unset | archive folder (`~` expanded); archiving is off when unset; see Archive below |
| `board.max_cell_height` | integer | unset | lines per board square at most; `1` keeps one-line glyphs; see Board size below |
| `assets` | table | `{}` | asset pack per game, e.g. `{ chess = "my.chess_art" }`; every game defaults to `"default"` |

A bad login or protocol mismatch stops auto-reconnect and notifies; fix the config and restart Neovim (or call `setup()` again).

### Colours

Board cells stack highlight groups, so a background layer and a foreground layer combine: a cell is `{square, piece, mark}`, and a last-move or check mark tints the square while the piece keeps its colour. Each group therefore sets only its role's attribute.

| Role | Groups |
|---|---|
| background | `GamesSquareLight`, `GamesSquareDark`, `GamesLastMove`, `GamesLastMoveDark` (layered over `GamesLastMove` on dark squares), `GamesCheck`, `GamesWin`, `GamesHit`, `GamesPresent`, `GamesMiss` |
| foreground | `GamesPieceWhite`, `GamesPieceBlack`, `GamesSeat1`-`GamesSeat4`, `GamesKey`, `GamesLabel`, `GamesMuted`, `GamesWon`, `GamesLost`, `GamesDrawn`, `GamesClockLow`, `GamesCheckSign` |
| ink (foreground) | `<background group>Ink` for each background group above, e.g. `GamesLastMoveInk`: dark or light text, whichever contrasts more with that background; stacked over it where text has no colour of its own (empty squares, the latest move in the move list) |
| badge (both) | `GamesYourTurn` |

White pieces are the hollow glyphs and black the solid ones under every palette.

Under `theme` each group copies one attribute from a colourscheme group (e.g. `GamesLastMove` takes `Search`'s background only) and is re-derived on every `:colorscheme`; of the two square shades, the lighter becomes `GamesSquareLight`. Inks are chosen after overrides and recomputed whenever the palette is applied (setup and every `:colorscheme`), not on a live `:hi` of a background group.

`highlights` in `setup()` always applies, cut to the group's role: background groups keep `bg`/`ctermbg`, foreground groups keep `fg`/`ctermfg`/`bold`/`italic`/`underline`, and a `{ link = ... }` is resolved to its target's attributes first; anything cut is reported once. A plain `:hi GamesX ...`, or a colourscheme's own `Games*` groups, are kept as they are: the palette's defaults never replace a group someone else has set since the plugin last defined it.

```lua
require("games").setup({
  palette = "vivid",
  highlights = { GamesLastMove = { bg = "#9bc700" } },
})
```

### Board size and asset packs

A board grows to fill its window: squares are as many lines tall as fit (twice as many columns wide), with the legend and move list beside it, and it refits when windows are resized, split or switched. When several windows show one board it fits the smallest. Squares 3 or more lines tall draw block-character art centred in them; smaller ones draw the one-line glyph. The board's size never depends on the game's state; the move list shows its latest moves within the board's height.

Under `ambiwidth=double` block characters draw two cells wide, so boards keep their one-line glyphs; any glyph that would widen (chess's `·`) falls back to the pack's ASCII one, and block separators to `|-+`. Changing `ambiwidth` redraws open boards.

Every glyph and sprite a game draws comes from an asset pack, `lua/games/assets/<game>/<pack>.lua`, so art can be swapped without touching game code. `assets = { chess = "name" }` picks a shipped pack by name or any Lua module by its module name. A pack returns:

```lua
{
  name = "mine",
  glyphs = { unicode = { K = { { h = 1, w = 1, lines = { "♔" } } }, ... }, ascii = { ... } },
  art = { K = { { h = 3, w = 4, lines = { " ▞▚ ", "▗▞▚▖", "▗▙▟▖" } }, ... }, ... },
}
```

Each entry is `h` lines of exactly `w` display columns. Art lines hold only spaces and U+2580-259F, uncoloured: colour comes from the board's highlight groups. Every key the game needs (chess: `K Q R B N P k q r b n p empty`) needs a one-line glyph in both glyph modes, as wide as the game's one-line cell (1 for chess); ascii glyphs must be printable ASCII, and one stands in for any unicode glyph that would widen under the current `ambiwidth`. `art` is optional per key, drawn under `glyphs = "unicode"` only. A pack that fails these checks, or raises an error while they run, is replaced by `"default"` with one notice.

## Commands

`:Games` with no argument opens a menu of the subcommands. Pickers use `vim.ui.select`, so a fuzzy picker plugin (telescope, snacks, fzf-lua) gives fuzzy search.

| Command | Does |
|---|---|
| `:Games challenge [game] [user…]` | challenge one or more users; prompts for missing opponents (pick "done" to stop), a game that fits that many players, and options |
| `:Games challenges` | pick a pending challenge: accept/decline (incoming) or cancel (outgoing) |
| `:Games list [ongoing\|finished]` | pick a game and open its board (default `ongoing`) |
| `:Games stats [user]` | W/L/D float (default: you) |
| `:Games leaderboard [game]` | leaderboard float, optionally for one game type |
| `:Games next` | open the next game waiting on your move |
| `:Games export` | archive the finished games the server lists for you (its 50 most recent); reports how many were written, already present, or had no view |

Incoming challenges and moves in boards you are not viewing arrive as `vim.notify` messages. Accepting a challenge opens the board when the game starts.

## Archive

With `archive = { dir = "~/games" }`, every game this client sees is written to

```
<dir>/<YYYY-MM-DD of the game's start, local time>/<game>/<game_id>/
```

| File | Contents |
|---|---|
| `summary.json` | the server's summary bytes, verbatim: game id, game, start and finish times, reason, each player's score |
| `summary.sha256` | the summary's sha256 as the server sent it, in `sha256sum` format, so `sha256sum -c summary.sha256` checks it |
| `final.json` | the finished game's update: the finishing broadcast if this client saw it, else a later read of the game, which carries the final board too |
| `events.jsonl` | every event this client saw, one JSON object per line, in order |
| `events.seq` | the seq of the last step whose events are in `events.jsonl`, so a restart does not repeat them |
| `<export>` | each file the server exports for the game, verbatim, e.g. chess's `game.pgn` |

The client checks the summary against its digest before writing and refuses a game whose digest does not match. Writes are atomic (temp file, then rename) and asynchronous; a game already archived is left as it is. Games that finished while Neovim was closed are archived by `:Games list finished` or `:Games export`; the server keeps each finished game's final board and exports, so only `events.jsonl` misses what this client did not see. Only the outcome is tamper-evident: send `summary.json` back with `game.verify {summary}` and the server confirms it matches the stored digest. A game id, game type, start time or export name that would place files outside the game's folder is refused and notified.

Known limitation: two Neovims logged in as the same user and archiving to the same `dir` can both append the same events to `events.jsonl`.

## Board keys

Buffer-local, normal mode, in a `games://` board buffer. Only keys valid for the current state are mapped; the footer lists them.

| Key | Action |
|---|---|
| `m` | enter a move (chess: SAN or UCI) |
| `R` | resign (confirms) |
| `D` | offer draw (the opponent's `D` agrees) |
| `X` | decline draw |
| `A` | abort |
| `C` | claim draw (chess), when a draw is claimable |
| `F` | flip board (chess) |
| `]g` | next game waiting on your move |
| `q` | close |

Chess move input is case-insensitive (`nf3`, `o-o`); the capital `B` is needed only when `bxc4` could be a bishop or a b-pawn.

## Adding a game

No core edits.

1. `server/games/<id>/`: implement `game.Rules[Opts, Action, View]` with typed options, actions, and view; its test calls `gametest.Run(t, <id>.Rules{}, scripts)`.
2. Add one line to `server/games/registry.go`.
3. `lua/games/games/<id>.lua`: a declarative layout plus an action table (20-60 lines, no window, buffer, keymap, or highlight code). The shell loads it by game id.

## Security

Transport is plain TCP, with no TLS. The username and password cross the wire in cleartext, and the NDJSON stream is readable by anyone on the path. The server binds to `127.0.0.1` by default. For private rooms, bind `--addr` to a Tailscale or WireGuard address, or keep the default and use an SSH tunnel (`ssh -L 7777:127.0.0.1:7777 host`), and use a password you use nowhere else. Failed logins are locked out per IP (5 failures in 5 min locks that IP out for 5 min). Passwords are stored as bcrypt hashes.

## Development

| Target | Runs |
|---|---|
| `make check` | lint, test-go, test-lua |
| `make test-go` | `go test -race -short ./...` plus chess perft |
| `make test-lua` | headless Neovim tests (`tests/lua/run.lua`); builds the server first |
| `make lint` | gofmt, `go vet`, `stylua --check`, lua-language-server |
| `make fmt` | gofmt and stylua |
