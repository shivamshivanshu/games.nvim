# Mancala

Two players, Kalah(6,4): six pits a side with four seeds each, and a store per player. Server: `server/games/mancala/`; board UI: `lua/games/games/mancala.lua`.

## Rules as implemented
- Seats are `south` and `north`. South moves first; the `first` option decides which player is south.
- Each seat owns the row of six pits in front of it and the store to its right. Each seat numbers its own pits 1 to 6 from its left, so pit 6 is next to its store.
- A move takes every seed from one of the mover's non-empty pits and sows them one per hole counter-clockwise: along the mover's pits toward its store, into its store, then along the opponent's pits. The opponent's store is skipped.
- A pit with 13 or more seeds laps the board and sows into the pit it was emptied from as well.
- **Extra turn:** if the last seed falls in the mover's own store, the mover moves again.
- **Capture:** if the last seed falls in one of the mover's own pits that was empty, and the opponent's pit facing it holds seeds, that last seed and every seed facing it go to the mover's store. South's pit 1 faces north's pit 6.
- **End:** after any move that leaves either side's six pits all empty, each seat's remaining seeds go to its own store and the game ends. The larger store wins (`more_seeds`); equal stores draw (`equal_seeds`).

## Rulings
| Question | Ruling | Why |
|---|---|---|
| Variant | Kalah(6,4), 48 seeds | spec §3.9 |
| Capture when the facing pit is empty | no capture: the last seed stays in its pit | the most common Kalah rule, as Wikipedia's Kalah rules state it ("and the opposite house contains seeds"); a capture of a single seed into the store would reward an empty facing pit |
| Sowing into the emptied pit on a lap | yes | Kalah skips only the opponent's store (Oware's skip of the origin is a different game) |
| When the game ends | after any move that empties either side, not only the side about to move | the Wikipedia Kalah rule; it ends the game without a forced pass |
| Leftover seeds | to the seat whose side they are on | spec §3.9 |
| Pit input | `"1"` to `"6"`, the mover's own pit, exactly as the catalogue lists them | catalogue values are what the parser accepts; relative numbering means the same key sows the same pit for either seat |
| First mover | south, chosen with the shared `first` option as in connect4 | one name for the first player in every view and event |

## Other endings (core)
- **Resign** (`R`, confirms): the opponent wins, reason `resign`.
- **Draw by agreement** (`D` to offer or accept, `X` to decline): reason `draw_agreed`.
- **Timeout:** the player whose clock reaches 0 loses, reason `timeout`.
- **Abort** (`A`): available until both players have moved once; result `aborted`.
- After any of these, `legal_pits` is empty, because mancala implements `game.Concluder`.

## Clock
The core runs it as for chess. On an extra turn the mover keeps the turn: its clock keeps running, and that sowing earns no increment, since the core adds the increment only when the turn leaves the mover.

## Options
| Option | Values | Default | Meaning |
|---|---|---|---|
| `first` | `random`, `me`, `opponent` | `random` | who plays south and moves first; `me` is the challenger |
| `time_control` | `none`, `3+2`, `5+0`, `10+5`, `15+10` | `10+5` | minutes + increment seconds |

## Controls and keys
Buffer-local, normal mode, in the board buffer. Only keys that are valid now are mapped; the footer lists them.

| Key | Action |
|---|---|
| `1`–`6` | sow that pit of yours, counted from your left; only on your move and only for pits with seeds (`1,3-6 sow`) |
| `R` | resign |
| `D` | offer a draw, or accept the opponent's |
| `X` | decline a pending draw offer |
| `A` | abort, until both players have moved |
| `]g` | next game waiting on your move |
| `q` | close |

## Display
- You sit at the bottom: your pits run 1 to 6 left to right, labelled under the board, into your store on the right. The opponent's pits run along the top from right to left into their store on the left, so sowing goes round counter-clockwise on screen too.
- A panel beside the board names who has the top row and who the bottom.
- Each cell shows its seed count in that player's header colour. A store is in parentheses `(12)`. On your move each pit you may sow is bracketed `[ 4]` and tinted with `GamesMovable`, from the view's `legal_pits`; the client decides nothing.
- Cells 5 lines tall and up draw the counts in block digits: a legal pit underlined, a store between bars.
- A move notifies as `bob sowed pit 2, capturing 8` or `alice sowed pit 3, and moves again`; the end as `bob collects the 36 seeds left on their side`.

## Wire
- **Action:** `{"kind": "sow", "pit": "3"}`.
- **View:** `{south: {pits, store}, north: {pits, store}, legal_pits}`. See `docs/protocol.md` and `docs/schema/games/mancala.json`.
- **Errors:**

| Case | Message |
|---|---|
| not `"1"` to `"6"` | `pit must be 1 to 6, got "<text>"` |
| an empty pit | `pit <n> is empty` |
| unknown action | `unknown action: <kind>` |
| out of turn, or the game is over | `not your turn` |

## Result and scoring
- **Reasons:** `more_seeds` (a win) and `equal_seeds` (a draw), plus the core's `resign`, `timeout`, `draw_agreed` and `aborted`.
- **Scores:** a win is +1, a loss −1 and a draw 0. An abort scores nothing. Mancala supplies no score of its own; the store counts are in the final view.
- There are no exports.

## Known limits
- Only Kalah(6,4): no other seed counts, no empty-capture variant, no pie rule.
- There is no "last move" marker on the board; the move's notice says which pit was sown.
- The clock display assumes the client's and the server's wall clocks agree.
