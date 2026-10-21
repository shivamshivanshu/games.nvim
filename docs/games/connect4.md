# Connect4

Two players, classic Connect Four. Server: `server/games/connect4/`; board UI: `lua/games/games/connect4.lua`.

## Rules as implemented
- The board is 7 columns × 6 rows. Rows count from 1 at the top, columns from 1 at the left.
- Seats are `red` and `yellow`. Red always moves first; the `first` option decides which player is red.
- A drop goes into a column and the disc comes to rest in the lowest empty cell.
- Four or more of one seat's discs in a row win: horizontally, vertically, or on either diagonal.
- After every drop the server checks for a win first, then for a full board. So a four made with the 42nd disc is a win, not a draw.
- A full board with no four is a draw.
- Every winning run through the last disc is reported whole. A run of five lists five cells, and one disc can complete two lines at once.

## Rulings
| Question | Ruling | Why |
|---|---|---|
| Board size | 7×6 fixed, no option | the classic board; spec §3.2 |
| First mover | red, the seat, always moves first | one name for "the first player" in every view and event |
| Pop Out or other variants | none | the spec asks for classic rules only |
| Column input type | text, `"1"` to `"7"` | the action catalogue takes text fields only; the server parses it |

## Other endings (core)
- **Resign** (`R`, confirms): the opponent wins, reason `resign`.
- **Draw by agreement** (`D` to offer or accept, `X` to decline): reason `draw_agreed`.
- **Timeout:** the player whose clock reaches 0 loses, reason `timeout`.
- **Abort** (`A`): available until both players have moved once. The result is `aborted`, with no winner and no score.
- After any of these, `legal_columns` is empty, because connect4's `Conclude` ends the game.

## Clock
- The core runs the clock exactly as it does for chess. It starts after both players have moved once, and the increment goes to the mover after each drop.
- The server decides timeouts; clients only show a countdown.

## Options
| Option | Values | Default | Meaning |
|---|---|---|---|
| `first` | `random`, `me`, `opponent` | `random` | who plays red and moves first; `me` is the challenger |
| `time_control` | `none`, `3+2`, `5+0`, `10+5`, `15+10` | `10+5` | minutes + increment seconds |

## Controls and keys
Keys are buffer-local, in Normal mode, in the board buffer. Only keys that are valid now are mapped, and the footer lists them.

| Key | Action |
|---|---|
| `1`–`7` | drop a disc into that column. These keys are shown only on your move, and only for columns with room. The footer collapses them, e.g. `1-7 drop` or `1-3,5-7 drop`. |
| `R` | resign |
| `D` | offer a draw, or accept the opponent's |
| `X` | decline a pending draw offer |
| `A` | abort, until both players have moved |
| `]g` | next game waiting on your move |
| `?` | help: legend and keys |
| `q` | close |

## Display
- Discs are drawn in their player's header colour.
- Unicode glyphs: red is solid `●`, yellow is hollow `○`, and an empty cell is `·`. The ascii glyphs are `X`, `O` and `.`.
- The seats differ by shape as well as colour, so colour is never the only cue.
- Larger windows draw block-art discs, 2 to 5 lines tall. Under `ambiwidth=double` no multi-line art is drawn, and the one-line glyphs stand in for it.
- The last disc is tinted with `GamesLastMove`. Once the game is won, the winning cells are tinted with `GamesWin`.

## Wire
- **Action:** `{"kind": "drop", "column": "4"}`.
- **View:** `{cells, last, winning_lines, legal_columns}`; cells are `{row, col}`, 1-based. See `docs/protocol.md` and `docs/schema/games/connect4.json`.
- **Rejections:**
  - `column must be 1 to 7, got "<text>"`;
  - `column <n> is full`;
  - an action of any other kind: `unknown action: <kind>`;
  - out of turn: `not_your_turn`.

## Result and scoring
- **Reasons:** `four_in_a_row` (a win) and `board_full` (a draw), plus the core's `resign`, `timeout`, `draw_agreed` and `aborted`.
- **Scores:** a win is +1, a loss −1 and a draw 0. An abort scores nothing. Connect4 supplies no score of its own.
- There are no exports.

## Known limits
- There are no takebacks and no variants (Pop Out, Five-in-a-Row, other sizes).
- The board is never flipped: gravity points down for both players.
- The clock display assumes the client's and the server's wall clocks agree.
