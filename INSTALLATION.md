# Installing games.nvim

How to set up the Go game server (the backend) and the Neovim plugin. Plugin options are under [Config](README.md#config) in the README.

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
2. Set `server.host` / `server.port` (see [Config](README.md#config)).
3. Run `:Games`.

## Host a server

```sh
go install github.com/shivamshivanshu/games.nvim/server/cmd/games-server@latest  # or: make build
games-server adduser alice                        # prompts for a password
games-server serve --addr 100.64.0.1:7777         # e.g. your Tailscale address
```

Details and flags are under [Server](#server) above.

## Run in Docker

The image runs `serve --addr 0.0.0.0:7777 --db /data/games.db` as the distroless `nonroot` user (uid 65532); the database lives on the `games-data` volume.

```sh
make docker                     # -> games-server:latest (override with IMAGE=...)
docker compose up -d            # publishes 127.0.0.1:7777
docker compose run --rm -it games adduser --db /data/games.db alice   # prompts for a password
docker compose run --rm -T games adduser --db /data/games.db --password-stdin bob < bob.pw
```

`GAMES_BIND` / `GAMES_PORT` choose the published address, e.g. `GAMES_BIND=100.64.0.1 docker compose up -d`.

Backup: stop first, then copy the volume out; never copy a live database, its `-wal` file holds recent writes (or use `sqlite3 games.db ".backup backup.db"` against a running one).

```sh
docker compose stop && docker compose cp games:/data/. ./games-backup/ && docker compose start
```

Gotchas:

- No TLS: keep the published port private (loopback, Tailscale, WireGuard); see [Security](README.md#security).
- Keep `/data` a mounted directory on local disk, not NFS: SQLite's WAL needs local file locking.
- Run one replica; two servers on one database corrupt each other's view of it.
- The per-IP login lockout sees every client as one IP behind rootless podman, Docker Desktop or a reverse proxy, so one client's bad passwords lock everyone out.
- Run `adduser` as the image's default user, never `--user root`: root-owned `-wal`/`-shm` files stop the server from opening the database.
