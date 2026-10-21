# Gomoku

Two players, freestyle Gomoku. Server: `server/games/gomoku/`, with the board and coordinates it shares with Reversi in `server/games/discs/` and `server/game/grid/coords.go`; board UI: `lua/games/games/gomoku.lua` over `lua/games/ui/discs.lua`.

## Rules as implemented
- The board is 15 × 15. Columns are lettered `a` to `o` from the left, rows numbered `1` to `15` from the top: `h8` is the centre.
- Seats are `black` and `white`. Black always moves first; the `first` option decides which player is black.
- A move places one stone on any empty cell. Stones never move and are never taken.
- Five or more of one seat's stones in a row win: horizontally, vertically, or on either diagonal.
- After every stone the server checks for a win first, then for a full board, so five made with the 225th stone is a win.
- A full board with no five is a draw.
- Every winning run through the last stone is reported whole: an overline of six lists six cells, and one stone can complete two lines at once.

## Rulings
| Question | Ruling | Why |
|---|---|---|
| Variant | freestyle: five **or more** wins, no restrictions on either side | user ruling, spec §3.9; uses `grid.WinningLines(k=5)` |
| Overlines | win | freestyle; standard Gomoku (exactly five) and Renju (black's 3-3, 4-4 and overline fouls) are not implemented |
| Opening rules (swap, swap2, pro) | none: black opens anywhere | not asked for; the first-move edge is accepted |
| Board size | 15 × 15, no option | user ruling |
| First mover | black, chosen with the shared `first` option as in connect4 and tictactoe | user ruling |
| Cell input | text such as `h8`, column letter then row number, either case, spaces around it ignored | the action catalogue takes text fields only; its pattern is the one the server parses with |
| Legal cells | the view lists every empty cell for the seat to move | C14: the engine sends legality; the board draws no marker for it, since every empty cell is legal |

## Other endings (core)
- **Resign** (`R`, confirms): the opponent wins, reason `resign`.
- **Draw by agreement** (`D` to offer or accept, `X` to decline): reason `draw_agreed`.
- **Timeout:** the player whose clock reaches 0 loses, reason `timeout`.
- **Abort** (`A`): available until both players have moved once; result `aborted`, no winner, no score.
- After any of these, `legal_cells` is empty, because gomoku's `Conclude` ends the game.

## Clock
The core runs it as for chess: it starts once both players have moved, and the increment goes to the mover after each stone. The server decides timeouts.

## Options
| Option | Values | Default | Meaning |
|---|---|---|---|
| `first` | `random`, `me`, `opponent` | `random` | who plays black and moves first; `me` is the challenger |
| `time_control` | `none`, `3+2`, `5+0`, `10+5`, `15+10` | `10+5` | minutes + increment seconds |

## Controls and keys
| Key | Action |
|---|---|
| `m` | place a stone: type the cell, e.g. `h8` |
| `R` | resign |
| `D` | offer a draw, or accept the opponent's |
| `X` | decline a pending draw offer |
| `A` | abort, until both players have moved |
| `]g` | next game waiting on your move |
| `?` | help: legend and keys |
| `q` | close |

The server checks the cell; a taken or off-board cell comes back as an error notice and the game does not move on.

## Display
- Columns are labelled `a`–`o` under the board, rows `1`–`15` beside it, row 1 at the top.
- Unicode glyphs: black is solid `●`, white hollow `○`, an empty cell `·`; ascii: `X`, `O`, `.`. Stones take the piece colours (`GamesPieceBlack`, `GamesPieceWhite`).
- The seats differ by shape as well as colour. The last stone is tinted with `GamesLastMove` and named beside the board (`last h8`); a win tints its cells with `GamesWin` and names each line (`line h8-l8`), so no state rests on colour alone.
- Larger windows draw block-art stones 2 to 5 lines tall; under `ambiwidth=double` the one-line glyphs stand in.

## Wire
- **Action:** `{"kind": "place", "cell": "h8"}`.
- **View:** `{cells, last, winning_lines, legal_cells}`; cells are `{row, col}`, 1-based. See `docs/protocol.md` and `docs/schema/games/gomoku.json`.
- **Event:** `placed {by, cell}`.
- **Rejections:**
  - `cell must be a1 to o15, a letter then a number, got "<text>"`;
  - `<cell> is taken`;
  - an action of any other kind: `unknown action: <kind>`;
  - out of turn: `not_your_turn`.

## Result and scoring
- **Reasons:** `five_in_a_row` (a win) and `board_full` (a draw), plus the core's `resign`, `timeout`, `draw_agreed` and `aborted`.
- **Scores:** win +1, loss −1, draw 0; an abort scores nothing. No exports.

## Known limits
- No Renju or standard-Gomoku variants, no opening rules, no other board sizes.
- Input is typed coordinates only; there is no cursor to move over the board.
- Each update to the seat to move lists every empty cell in `legal_cells`, up to 225 of them.
