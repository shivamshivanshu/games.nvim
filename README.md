# games.nvim

Turn-based games with friends inside Neovim. A Go server owns all state and rules; the Lua plugin renders boards and forwards your input over TCP (NDJSON).

Setup: [INSTALLATION.md](INSTALLATION.md).

## Games

| Game | Players | Rules |
|---|---|---|
| Chess | 2 | [docs/games/chess.md](docs/games/chess.md) |
| Tic-Tac-Toe | 2 | [docs/games/tictactoe.md](docs/games/tictactoe.md) |
| Connect4 | 2 | [docs/games/connect4.md](docs/games/connect4.md) |
| Checkers | 2 | [docs/games/checkers.md](docs/games/checkers.md) |
| Ludo | 2–4 | [docs/games/ludo.md](docs/games/ludo.md) |
| Wordle | 1 (solo) | [docs/games/wordle.md](docs/games/wordle.md) |
| Sudoku | 1 (solo) | [docs/games/sudoku.md](docs/games/sudoku.md) |
| Poker | 2–6 | [docs/games/poker.md](docs/games/poker.md) |
| Gomoku | 2 | [docs/games/gomoku.md](docs/games/gomoku.md) |
| Reversi | 2 | [docs/games/reversi.md](docs/games/reversi.md) |
| Chess puzzles and study | 1 (solo) | [docs/games/chess_puzzle.md](docs/games/chess_puzzle.md) |
| Mastermind | 1 (solo) | [docs/games/mastermind.md](docs/games/mastermind.md) |
| Hangman | 1 (solo) | [docs/games/hangman.md](docs/games/hangman.md) |
| Minesweeper | 1 (solo) | [docs/games/minesweeper.md](docs/games/minesweeper.md) |
| 15-puzzle | 1 (solo) | [docs/games/fifteen.md](docs/games/fifteen.md) |
| 2048 | 1 (solo) | [docs/games/g2048.md](docs/games/g2048.md) |
| Mancala | 2 | [docs/games/mancala.md](docs/games/mancala.md) |
| Yahtzee | 1–4 | [docs/games/yahtzee.md](docs/games/yahtzee.md) |
| Liar's Dice | 2–6 | [docs/games/liars_dice.md](docs/games/liars_dice.md) |
| Uno | 2–6 | [docs/games/uno.md](docs/games/uno.md) |
| Hearts | 4 | [docs/games/hearts.md](docs/games/hearts.md) |
| Battleship | 2 | [docs/games/battleship.md](docs/games/battleship.md) |
| Chess960 | 2 | [docs/games/chess960.md](docs/games/chess960.md) |
| Dots and Boxes | 2 | [docs/games/dots_and_boxes.md](docs/games/dots_and_boxes.md) |
| Pac-Man | 1 (solo) | [docs/games/pacman.md](docs/games/pacman.md) |

## Config

| Option | Type | Default | Notes |
|---|---|---|---|
| `server.host` | string | `"127.0.0.1"` | |
| `server.port` | integer | `7777` | |
| `username` | string | prompted | |
| `password` | string or `fun(): string` | prompted | a function is called at each connect |
| `autoconnect` | boolean | `true` | connect at startup; needs `username` |
| `glyphs` | `"unicode"` or `"ascii"` | `"unicode"` | piece glyphs |
| `palette` | `"theme"` or `"vivid"` | `"theme"` | colours from your colourscheme, or a fixed lichess-style board (needs `termguicolors`, else `theme`) |
| `highlights` | table | `{}` | `{ GamesX = {...} }` overrides; see [Colours](#colours) |
| `archive.dir` | string | unset | turns on the [Archive](#archive) |
| `board.max_cell_height` | integer | unset | most lines per square; `1` keeps one-line glyphs |
| `assets` | table | `{}` | [asset pack](#asset-packs) per game, e.g. `{ chess = "my.chess_art" }` |
| `picker` | `"snacks"`, `"telescope"`, `"fzf"` or `"select"` | first installed | the [Finder](#finder)'s backend; one not installed falls back to `vim.ui.select` |
| `keys` | `{ dashboard?, find? }` | `{}` | global keys for `:Games` and `:Games find`, e.g. `{ dashboard = "<leader>gg" }` |
| `puzzles.max_entries` | integer | `5000` | entries kept per imported puzzle file |

A bad login or protocol mismatch stops reconnecting; fix the config, then restart Neovim or call `setup()` again.

### Colours

A board cell stacks a background group (the square, or a last-move or check tint) under a foreground group (the piece), so each group sets only its role's attributes.

| Role | Groups |
|---|---|
| background | `GamesSquareLight`, `GamesSquareDark`, `GamesLastMove`, `GamesLastMoveDark` (over `GamesLastMove` on dark squares), `GamesCheck`, `GamesWin`, `GamesHit`, `GamesPresent`, `GamesMiss`, `GamesMovable` |
| foreground | `GamesPieceWhite`, `GamesPieceBlack`, `GamesSeat1`-`GamesSeat4`, `GamesKey`, `GamesLabel`, `GamesMuted`, `GamesWon`, `GamesLost`, `GamesDrawn`, `GamesClockLow`, `GamesCheckSign`, `GamesDashSection`, `GamesGiven` (sudoku givens), `GamesCardRed`, `GamesCardYellow`, `GamesCardGreen`, `GamesCardBlue` (cards), `GamesPegRed`, `GamesPegGreen`, `GamesPegBlue`, `GamesPegYellow`, `GamesPegOrange`, `GamesPegPurple` (mastermind), `GamesPacman`, `GamesPacmanWall`, `GamesGhostBlinky`, `GamesGhostPinky`, `GamesGhostInky`, `GamesGhostClyde`, `GamesGhostFrightened` (pacman) |
| ink (foreground) | `<background group>Ink`, e.g. `GamesLastMoveInk`: whichever of dark or light text contrasts more with that background, for text with no colour of its own |
| badge (both) | `GamesYourTurn` |

White pieces are the hollow glyphs, black the solid ones. Under `theme` each group copies one attribute from a colourscheme group (e.g. `GamesLastMove` takes `Search`'s background) and follows `:colorscheme`. A `highlights` override is cut to its group's role (`bg`/`ctermbg` for background; `fg`/`ctermfg`/`bold`/`italic`/`underline` for foreground), with a notice for what was cut. Groups set by `:hi` or your colourscheme are never overwritten.

```lua
require("games").setup({
  palette = "vivid",
  highlights = { GamesLastMove = { bg = "#9bc700" } },
})
```

### Board size

A board fills its window and refits on resize, fitting the smallest window when several show it. Squares are as many lines tall as fit and twice as wide; at 3 or more lines they draw block-character art instead of the one-line glyph. Under `ambiwidth=double` boards keep one-line glyphs, and any glyph that would widen falls back to ASCII.

### Asset packs

Every glyph and sprite comes from an asset pack, `lua/games/assets/<game>/<pack>.lua`. `assets = { chess = "name" }` picks a shipped pack by name or any Lua module.

```lua
{
  name = "mine",
  glyphs = { unicode = { K = { { h = 1, w = 1, lines = { "♔" } } }, ... }, ascii = { ... } },
  art = { K = { { h = 3, w = 4, lines = { " ▞▚ ", "▗▞▚▖", "▗▙▟▖" } }, ... }, ... },
}
```

- Each entry is `h` lines of exactly `w` display columns.
- Every key the game needs (chess: `K Q R B N P k q r b n p empty`) needs a one-line glyph in both `unicode` and `ascii` (printable ASCII), as wide as the game's cell.
- `art` is optional per key and drawn only under `glyphs = "unicode"`: spaces and U+2580-259F, uncoloured.
- Pictures drawn beside the board (hangman: `gallows:0`-`gallows:6`) are glyph entries of any size.

A pack that fails these checks is replaced by `"default"`, with one notice.

## Commands

`:Games` opens the [Dashboard](#dashboard).

| Command | Does |
|---|---|
| `:Games challenge [game] [user…]` | challenge one or more users; prompts for what is missing |
| `:Games challenges` | pending challenges (the `requests` finder) |
| `:Games solo [game]` | start a one-player game |
| `:Games list [ongoing\|finished]` | your games of that status (default `ongoing`) |
| `:Games stats [user]` | wins, losses and draws (default: you) |
| `:Games leaderboard [game]` | the leaderboard; the all-games total leaves out solo games |
| `:Games next` | open the next game waiting on your move |
| `:Games find [source]` | the [Finder](#finder) |
| `:Games export` | archive your 50 most recent finished games |
| `:Games import <file> [set] [max]` | import a Lichess puzzle `.csv` or a `.pgn` into a puzzle set (default: the file's name), keeping the first `max` entries; works offline; a PGN game without a `FEN` header is a study |
| `:Games puzzle [set]` | start the next entry of a puzzle set (default: the latest imported) |

Options are asked one at a time, default first; cancelling one cancels the start. Challenges, and moves in boards you are not viewing, arrive as notifications.

## Dashboard

A float that follows the server live while open:

| Section | Lists |
|---|---|
| Requests | pending challenges, incoming first |
| Your move | ongoing games waiting on you |
| Ongoing | ongoing games waiting on someone else |
| Recent | your last 5 finished games |
| New | every game type, numbered `1`-`9` |

| Key | On the line under the cursor |
|---|---|
| `<CR>` | open the game, start a game of that type, or act on the request |
| `a` / `d` | accept / decline (or cancel your own) a request |
| `r` | resign, or give up a solo game (confirms) |
| `1`-`9` | start a game of that New type |
| `/` | open the finder on this section |
| `R` | refresh |
| `?` | list every key |
| `q`, `<Esc>` | close |

It closes when you focus another window. While disconnected it shows `reconnecting…` and refreshes once logged in again.

## Finder

`:Games find [source]` lists one source, previewing the item under the cursor:

| Source | Lists | `<CR>` | Other actions |
|---|---|---|---|
| `requests` | pending challenges | asks which action | accept, decline, cancel |
| `games` | ongoing games (your move first), then finished ones | open | resign or give up |
| `types` | game types, with their options | start | leaderboard |
| `players` | other users, online first, ranked | challenge | stats |
| `puzzles` | saved puzzle sets, with their next entry | start the next entry | restart |

It uses [snacks.picker](https://github.com/folke/snacks.nvim), [telescope](https://github.com/nvim-telescope/telescope.nvim) or [fzf-lua](https://github.com/ibhagwan/fzf-lua), the first one installed (or `picker`), where `<M-CR>` picks among an item's actions. Without any of them it uses `vim.ui.select`, with no preview.

## Archive

With `archive = { dir = "~/games" }`, every game this client sees is written to `<dir>/<start date>/<game>/<game_id>/`:

| File | Contents |
|---|---|
| `summary.json` | the server's summary: players, times, reason, scores |
| `summary.sha256` | its digest; `sha256sum -c summary.sha256` checks it |
| `final.json` | the finished game, final board included |
| `events.jsonl` | every event this client saw, one per line |
| `events.seq` | the last archived step, so a restart does not repeat events |
| `<export>` | each file the server exports, e.g. chess's `game.pgn` |

Games that finished while Neovim was closed are archived by `:Games list finished` or `:Games export`, but their `events.jsonl` lacks what this client did not see. A summary whose digest does not match is refused; `game.verify {summary}` asks the server to confirm one. Two Neovims logged in as the same user and sharing a `dir` can duplicate events.

## Board keys

Buffer-local in a `games://` board. Only keys valid now are mapped, and the footer lists them.

| Key | Action |
|---|---|
| `m` | enter a move (formats below) |
| `R` | resign, or give up a solo game (confirms) |
| `D` / `X` | offer or accept a draw / decline one |
| `A` | abort |
| `C` | claim a draw (chess) |
| `F` | flip the board (chess, checkers) |
| `]g` | next game waiting on your move |
| `?` | what each board mark means, and every key bound now |
| `q` | close |

| Game | Input |
|---|---|
| chess | `m`: SAN or UCI, case-insensitive (`nf3`, `o-o`); capital `B` only when `bxc4` is ambiguous |
| chess puzzle | as chess; a wrong move ends a puzzle; `n` reveals the move in study mode, counting as a miss |
| checkers | `m`: squares 1-32, `11-15`, or every landing of a capture, `15x24x31`; movable pieces are tinted |
| tictactoe | `1`-`9`: that cell, phone layout |
| ludo | `r` rolls; `1`-`4` moves that token |
| sudoku | `m`, then `53 7` sets row 5, column 3 to 7 (`0` clears); `x`, then `53` clears it |
| wordle | `m`: a five-letter guess |
| mastermind | `m`: 4 colour letters from `rgbyop`, e.g. `rgby` |
| hangman | `m`: a letter or the whole word |
| minesweeper | `m` reveal, `f` flag or unflag, `c` chord a number; cells like `c12` (row c, column 12) |
| fifteen | `m` and a tile number, or `h` `j` `k` `l` |
| 2048 | `h` `j` `k` `l` or the arrows |
| mancala | `1`-`6`: sow that pit, counted from your left |
| pacman | `h` `j` `k` `l` or the arrows turn (and start a paused game); `p` pauses |

The chess header shows, after each clock, the material that side is up and `+N` for the side ahead.

## Adding a game

No core edits.

1. `server/games/<id>/`: implement `game.Rules[Opts, Action, View]`; its test calls `gametest.Run(t, <id>.Rules{}, scripts)`.
2. Add one line to `server/games/registry.go`, list the game's `Events` (and any new `Reasons`) in its `Info`, and run `make schema`.
3. `lua/games/games/<id>.lua`: a declarative layout plus an action table, with no window, buffer, keymap or highlight code.

## Security

Transport is plain TCP: usernames, passwords and games cross the wire in cleartext. The server binds `127.0.0.1` by default; to share it, bind `--addr` to a Tailscale or WireGuard address or use an SSH tunnel (`ssh -L 7777:127.0.0.1:7777 host`), and use a password you use nowhere else. Passwords are stored as bcrypt hashes. Five failed logins in 5 minutes lock that IP out for 5 minutes.

## Development

Protocol, for writing another client: [docs/protocol.md](docs/protocol.md), with JSON Schema in `docs/schema/` and replayable transcripts in `docs/conformance/`.

| Target | Runs |
|---|---|
| `make check` | lint, schema-check, test-go, test-lua |
| `make schema` | regenerates `docs/schema/` from the Go types |
| `make schema-check` | fails when `docs/schema/` is out of date |
| `make test-go` | `go test -race -short ./...` plus chess perft |
| `make test-lua` | headless Neovim tests (`tests/lua/run.lua`); builds the server first |
| `make lint` | gofmt, `go vet`, `stylua --check`, lua-language-server |
| `make fmt` | gofmt and stylua |
| `make smoke-finders` | drives the finder headless in each picker; needs `fzf` on `PATH` and fetches the pickers at pinned commits into `PICKERS_PACK` (default `.pickers/`) |
