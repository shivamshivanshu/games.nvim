# Reversi

Two players, Reversi (Othello rules) on 8 × 8. Server: `server/games/reversi/`, with the board and coordinates it shares with Gomoku in `server/games/discs/` and `server/game/grid/coords.go`; board UI: `lua/games/games/reversi.lua` over `lua/games/ui/discs.lua`.

## Rules as implemented
- The board is 8 × 8. Columns are lettered `a` to `h` from the left, rows numbered `1` to `8` from the top.
- It starts with white on `d4` and `e5`, black on `e4` and `d5`. Black moves first, so its opening moves are `d3`, `c4`, `f5` and `e6`.
- Seats are `black` and `white`; the `first` option decides which player is black.
- A move places a disc on an empty cell so that it closes at least one straight run of the opponent's discs, in any of the 8 directions, with another of the mover's discs. Every run it closes is turned over. A cell that closes none is illegal.
- After each move the turn passes to the opponent if it has a legal move. If it has none but the mover does, the opponent **passes**: the server applies the pass itself and sends a `passed` event, and the mover moves again. There is no pass action.
- When neither seat can move (a full board, a wipeout, or both blocked) the game ends. More discs wins; equal counts draw.

## Rulings
| Question | Ruling | Why |
|---|---|---|
| Board and start | standard 8 × 8, the usual four discs, black first | user ruling, spec §3.9 |
| Passing | automatic, with a `passed {by}` event; the client never sends a pass | user ruling: a pass is never a choice |
| End | when neither seat can move; the count decides | user ruling |
| Empty cells at the end | not added to the winner's count | the simple count the ruling names; only the reported score would differ, never the winner |
| First mover | black, chosen with the shared `first` option as in connect4 and tictactoe | user ruling |
| Cell input | text such as `d3`, column letter then row number, either case, spaces around it ignored | the action catalogue takes text fields only; its pattern is the one the server parses with |

## Other endings (core)
- **Resign** (`R`, confirms): the opponent wins, reason `resign`.
- **Draw by agreement** (`D` to offer or accept, `X` to decline): reason `draw_agreed`.
- **Timeout:** the player whose clock reaches 0 loses, reason `timeout`.
- **Abort** (`A`): available until both players have moved once; result `aborted`.
- After any of these, `legal_cells` is empty, because reversi's `Conclude` ends the game.

## Clock
The core runs it as for chess. When the opponent passes, the mover keeps the turn: its clock keeps running, and that move earns no increment, since the core adds the increment only when the turn leaves the mover.

## Options
| Option | Values | Default | Meaning |
|---|---|---|---|
| `first` | `random`, `me`, `opponent` | `random` | who plays black and moves first; `me` is the challenger |
| `time_control` | `none`, `3+2`, `5+0`, `10+5`, `15+10` | `10+5` | minutes + increment seconds |

## Controls and keys
| Key | Action |
|---|---|
| `m` | place a disc: type the cell, e.g. `d3` |
| `R` | resign |
| `D` | offer a draw, or accept the opponent's |
| `X` | decline a pending draw offer |
| `A` | abort, until both players have moved |
| `]g` | next game waiting on your move |
| `q` | close |

## Display
- Columns are labelled `a`–`h` under the board, rows `1`–`8` beside it, row 1 at the top.
- Unicode glyphs: black solid `●`, white hollow `○`, empty `·`; ascii: `X`, `O`, `.`. Discs take the piece colours.
- On your move each cell you may play shows `+` and is tinted with `GamesMovable`; the cells come from the view's `legal_cells`, so the client decides nothing.
- Each header line shows that player's disc count (`12 discs`).
- The last disc is tinted with `GamesLastMove` and named beside the board (`last d3`).
- A move notifies as `alice placed d3, flipping 1`; a pass as `bob has no move and passes`.
- Larger windows draw block-art discs; under `ambiwidth=double` the one-line glyphs stand in.

## Wire
- **Action:** `{"kind": "place", "cell": "d3"}`.
- **View:** `{cells, last, legal_cells, discs: {black, white}}`; cells are `{row, col}`, 1-based. See `docs/protocol.md` and `docs/schema/games/reversi.json`.
- **Events:** `placed {by, cell, flipped}`, then `passed {by}` in the same update when the opponent is blocked.
- **Rejections:**
  - `cell must be a1 to h8, a letter then a number, got "<text>"`;
  - `<cell> is taken`;
  - `<cell> flips no disc; a move must flip at least one`;
  - an action of any other kind: `unknown action: <kind>`;
  - out of turn: `not_your_turn`.

## Result and scoring
- **Reasons:** `more_discs` (a win) and `equal_discs` (a draw), both when neither seat can move; plus the core's `resign`, `timeout`, `draw_agreed` and `aborted`.
- **Scores:** win +1, loss −1, draw 0; the disc margin is not scored. No exports.

## Known limits
- Input is typed coordinates only; there is no cursor to move over the board.
- The flipped discs are named in the event, not marked on the board.
