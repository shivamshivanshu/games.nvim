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
| `glyphs` | `"unicode"` or `"ascii"` | `"unicode"` | piece glyphs |

A bad login or protocol mismatch stops auto-reconnect and notifies; fix the config and restart Neovim (or call `setup()` again).

## Commands

`:Games` with no argument opens a menu of the subcommands. Pickers use `vim.ui.select`, so a fuzzy picker plugin (telescope, snacks, fzf-lua) gives fuzzy search.

| Command | Does |
|---|---|
| `:Games challenge [user] [game]` | challenge someone; prompts for missing user, game, and options |
| `:Games challenges` | pick a pending challenge: accept/decline (incoming) or cancel (outgoing) |
| `:Games list [ongoing\|finished]` | pick a game and open its board (default `ongoing`) |
| `:Games stats [user]` | W/L/D float (default: you) |
| `:Games leaderboard [game]` | leaderboard float, optionally for one game type |
| `:Games next` | open the next game waiting on your move |

Incoming challenges and moves in boards you are not viewing arrive as `vim.notify` messages. Accepting a challenge opens the board when the game starts.

## Board keys

Buffer-local, normal mode, in a `games://` board buffer. Only keys valid for the current state are mapped; the footer lists them.

| Key | Action |
|---|---|
| `m` | enter a move (chess: SAN or UCI) |
| `R` | resign (confirms) |
| `D` | offer draw (the opponent's `D` agrees) |
| `X` | decline draw |
| `A` | abort |
| `C` | claim draw (chess) |
| `F` | flip board (chess) |
| `]g` | next game waiting on your move |
| `q` | close |

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
