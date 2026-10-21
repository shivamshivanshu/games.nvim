# 15-puzzle

Solo: slide numbered tiles into the one blank square until they are in order. Server: `server/games/fifteen/`; board UI: `lua/games/games/fifteen.lua`.

## Rules as implemented
- One seat, `solver`. Start with `:Games solo fifteen`.
- The board is 4×4 with tiles 1-15, or 3×3 with tiles 1-8, and one blank square.
- A move slides a tile next to the blank (above, below, left or right; never diagonal) into it.
- Two ways to name a move, the same move either way:
  - **slide** a tile by its number;
  - **move** by direction: the tile moves that way into the blank, so `up` moves the tile below the blank.
- The puzzle is solved, reason `solved`, when the tiles read 1 to the last in rows from the top left with the blank in the bottom right.
- **The move count is shown but not scored.** The view carries it as `moves` and the board shows it beside the tiles; as for every solo game the result is +1 when solved and −1 otherwise.

## Shuffle
The shuffle is drawn when the game starts, from the game's seed, so a restart rebuilds it.
1. A uniformly random arrangement of the tiles and the blank.
2. If it cannot be solved, the first two tiles that are not the blank are swapped, which makes it solvable. An arrangement is solvable exactly when the parity of its permutation (blank included) equals the parity of the blank's distance from the bottom right corner, because every slide flips both. A test checks this against all 181,440 arrangements reachable on 3×3.
3. If that leaves it solved, it is drawn again, so a game never starts solved.

## Rulings
- The direction names the way the tile moves, as in most sliding-puzzle apps, not the way the blank moves.
- The view lists the movable tiles with the way each would move (`movable`), so the client binds only the direction keys that can act and frames the tiles that can slide. No rule runs in the client.
- The move count is the game's score in the view. The archive's outcome score stays the solo default below, so the leaderboard means the same for every solo game.
- No clock and no undo; each slide counts.

## Other endings (core)
- Give up (`R`, confirms): a loss with no winner, reason `resign`.
- Abort (`A`): only before your first move; result `aborted`, scores nothing.

## Options
| Option | Values | Default | Meaning |
|---|---|---|---|
| `size` | `3x3`, `4x4` | `4x4` | tiles 1-8 or 1-15 |

## Controls and keys
Buffer-local, normal mode, in the board buffer.

| Key | Action |
|---|---|
| `m` | slide a tile: type its number, e.g. `7` |
| `h` `j` `k` `l` | slide the tile that moves left, down, up or right, when one can |
| `R` | give up |
| `A` | abort, before your first move |
| `]g` | next game waiting on your move |
| `q` | close |

A panel beside the board shows the move count. A movable tile is bracketed `[ 7]` and tinted (`GamesMovable`), so that never rests on colour alone; once solved every tile is tinted (`GamesWin`). In a roomy window the tiles are drawn in block digits, a movable one underlined.

## Errors
| Case | Message |
|---|---|
| text not a tile number 1-15 | `want a tile number, e.g. "7"; got "<text>"` |
| a tile the board does not have | `no tile N on a 3x3 board` |
| a tile not next to the blank | `tile N is not next to the blank` |
| a direction no tile can move | `no tile can move <direction>` |
| not up, down, left or right | `want up, down, left or right; got "<text>"` |
| unknown action | `unknown action: <kind>` |
| the game is over | `not your turn` |

## Scoring and result
Solved +1, gave up -1, abort 0. Solo games count in the fifteen records but not in the all-games leaderboard.

## Known limits
- Input is typed or the direction keys; there is no cursor or mouse.
- The conformance transcript gives up rather than solving, because the hub picks a random seed; solving is covered by the Go rules tests and the smoke test.
