# Installation

The Go server holds every game's state; the Neovim plugin connects to it. Plugin options are under [Config](README.md#config).

## Requirements

| Part | Needs |
|---|---|
| Server | Go 1.26+ |
| Plugin | Neovim 0.11+ (not checked at runtime) |
| Development | `stylua`, `lua-language-server` (`LUALS`, default mason path) |

## Server

```sh
make build                       # -> bin/games-server
# or: go install github.com/shivamshivanshu/games.nvim/server/cmd/games-server@latest
bin/games-server adduser alice   # prompts for a password
bin/games-server serve
```

Accounts exist only through `adduser [--db games.db] [--password-stdin] <name>`; use the same `--db` as `serve`.

| `serve` flag | Default | Meaning |
|---|---|---|
| `--addr` | `127.0.0.1:7777` | listen address, e.g. your Tailscale IP to host friends |
| `--db` | `games.db` | SQLite file |
| `--heartbeat` | `15s` | heartbeat interval |
| `--idle` | `45s` | drop a silent client after this |

There is no TLS: read [Security](README.md#security) before exposing the port.

### Docker

The image runs `serve --addr 0.0.0.0:7777 --db /data/games.db` as the distroless `nonroot` user (uid 65532), with the database on the `games-data` volume.

```sh
make docker                    # -> games-server:latest (IMAGE=... to override)
docker compose up -d           # publishes 127.0.0.1:7777; GAMES_BIND / GAMES_PORT change it
docker compose run --rm -it games adduser --db /data/games.db alice
docker compose run --rm -T games adduser --db /data/games.db --password-stdin bob < bob.pw
```

Back up with the server stopped, since the `-wal` file holds recent writes (or run `sqlite3 games.db ".backup backup.db"` against a live one):

```sh
docker compose stop && docker compose cp games:/data/. ./games-backup/ && docker compose start
```

- Keep `/data` on local disk, not NFS: SQLite's WAL needs local file locks.
- Run one replica per database.
- Run `adduser` as the image's user, never `--user root`: root-owned `-wal`/`-shm` files stop the server opening the database.
- Behind rootless podman, Docker Desktop or a reverse proxy every client shares one IP, so one client's failed logins lock everyone out.

## Plugin

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

`username` and `password` are optional: `:Games` prompts for missing ones (the password hidden) and keeps them for the session. A `password` function is called at each connect, so the secret can stay out of your dotfiles.

Other managers load the plugin, then call `require("games").setup({ ... })`:

```lua
-- packer.nvim
use({ "shivamshivanshu/games.nvim", config = function() require("games").setup({ ... }) end })

-- mini.deps
MiniDeps.add("shivamshivanshu/games.nvim")
```

vim-plug: `Plug 'shivamshivanshu/games.nvim'`. Native packages: clone into `~/.local/share/nvim/site/pack/games/start/games.nvim`, then `:helptags ALL`.

To join a server, get an account from its host, set `server.host` and `server.port`, and run `:Games`. `:checkhealth games` diagnoses the setup; `:help games` is the manual.
