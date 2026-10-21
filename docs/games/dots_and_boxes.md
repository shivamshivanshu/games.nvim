# Dots and Boxes

Two players take turns drawing a line between two adjacent dots; whoever completes a box claims it and moves again. Server: `server/games/dotsandboxes/`; board UI: `lua/games/games/dots_and_boxes.lua`.

## Rules as implemented
- The board is a square of 3, 5 or 7 boxes a side (the `size` option), so 4, 6 or 8 dots a side. Every edge starts undrawn.
- Seats are `p1` and `p2`. `p1` moves first; the `first` option decides which player is `p1`.
- A move draws one undrawn edge between two dots next to each other across or down. Edges on the border count like any other.
- **Another move:** an edge that completes one box, or the two boxes either side of it, gives those boxes to the mover, and the mover draws again. An edge that completes nothing passes the move.
- **Moving is compulsory:** a player cannot pass, and while any edge is undrawn there is one to draw.
- **End:** the move that completes the last box ends the game. The player with more boxes wins (`more_boxes`). Equal boxes would draw (`equal_boxes`), but every offered size has an odd number of boxes, so a natural draw cannot happen.

## Rulings
| Question | Ruling | Why |
|---|---|---|
| Sizes | 3, 5 or 7 boxes a side, default 5 | spec §3.9; 5 is a game of a few minutes |
| Completing a box | earns another move, also after completing two at once, and again after each completion in a chain | spec §3.9; the standard rule |
| When the game ends | once every box is complete, never early when one side holds a majority | the standard rule; the final score is shown in full |
| Who moves first | `p1`, chosen with the shared `first` option | one name for the first player in every view and event, as in battleship |
| Edge names | the dot at the edge's left or top end, then `h` lying across or `v` lying down, e.g. `b2h` joins b2 and c2, `b2v` joins b2 and b3 | one name per edge; dots are named like cells, column letter from `a` at the left and row number from 1 at the top |
| Catalogue pattern | the 7-box board's, for every size; an edge off a smaller board is refused when it is applied | the catalogue is one per game while the board size is an option; the parser and the pattern are one regexp |
| Drawn edges | the view records which seat drew each edge; the board draws them all alike | no state may rest on colour alone, and who drew an edge decides nothing |

## Other endings (core)
- **Resign** (`R`, confirms): the opponent wins, reason `resign`.
- **Draw by agreement** (`D` to offer or accept, `X` to decline): reason `draw_agreed`.
- **Timeout:** the player whose clock reaches 0 loses, reason `timeout`.
- **Abort** (`A`): available until both players have moved once; result `aborted`.
- After any of these, `legal_edges` is empty, because dots_and_boxes implements `game.Concluder`.

## Clock
The core runs it as for chess. After completing a box the mover keeps the turn: its clock keeps running, and that move earns no increment, since the core adds the increment only when the turn leaves the mover.

## Options
| Option | Values | Default | Meaning |
|---|---|---|---|
| `size` | `3`, `5`, `7` | `5` | boxes a side |
| `first` | `random`, `me`, `opponent` | `random` | who is `p1` and moves first; `me` is the challenger |
| `time_control` | `none`, `3+2`, `5+0`, `10+5`, `15+10` | `10+5` | minutes + increment seconds |

## Controls and keys
Buffer-local, normal mode, in the board buffer. Only keys that are valid now are mapped; the footer lists them.

| Key | Action |
|---|---|
| `h` `j` `k` `l` | move the cursor one dot left, down, up or right, on your move; it stays on the board's edges |
| `r` | turn the cursor's edge between across (from its dot to the right) and down (from its dot downward) |
| `<CR>` | draw the edge under the cursor; bound only when the view offers that edge |
| `R` | resign |
| `D` | offer a draw, or accept the opponent's |
| `X` | decline a pending draw offer |
| `A` | abort, until both players have moved |
| `]g` | next game waiting on your move |
| `q` | close |

The cursor starts on `a1h`, the top left edge, and keeps its place between moves.

## Display
- Dots `·` (ascii `+`) sit at the corners of the boxes; a drawn edge is `─` or `│` (ascii `-` or `|`), an undrawn one is blank. Row numbers label the dots on the left and column letters along the bottom, as edges are typed.
- The latest edge is heavy, `━` or `┃` (ascii `=` or `#`), on `GamesLastMove`.
- On your move the cursor's edge is dashed, `┄` or `┆` (ascii `.` or `:`), on `GamesMovable` when you may draw it; on an edge already drawn it shows `×` (ascii `x`) on `GamesCheck`, the panel reads `at b2h: drawn`, and `<CR>` is unbound. Legality comes from the view's `legal_edges`; the client decides nothing.
- A completed box shows its owner's glyph, `●` for `p1` and `○` for `p2` (ascii `X` and `O`), in the colour the header gives that player; cells 3 lines tall and up draw it in block art.
- Each header line shows the player's glyph and boxes, e.g. `● 3 boxes`.
- A panel beside the board says whose move it is, the edge under the cursor, and the latest edge.
- A move notifies as `alice drew c2h`, or `bob drew b1v, completing 1 box, and moves again`.

## Wire
- **Action:** `{"kind": "draw", "edge": "b2h"}`.
- **View:** `{across, down, boxes, scores, last, legal_edges}`. See `docs/protocol.md` and `docs/schema/games/dots_and_boxes.json`.
- **Errors:**

| Case | Message |
|---|---|
| not an edge of the 7-box board | `edge must be a dot then h or v, such as b2h, got "<text>"` |
| an edge off this board | `<edge> is off the board of <n> boxes a side` |
| an edge already drawn | `<edge> is already drawn` |
| unknown action | `unknown action: <kind>` |
| out of turn, or the game is over | `not your turn` |

## Result and scoring
- **Reasons:** `more_boxes` (a win) and `equal_boxes` (a draw, unreachable at the offered sizes), plus the core's `resign`, `timeout`, `draw_agreed` and `aborted`.
- **Scores:** a win is +1, a loss −1 and a draw 0. An abort scores nothing. The box counts are in the final view.
- There are no exports.

## Known limits
- Only square boards of 3, 5 or 7 boxes; no rectangular boards, no early end on a majority.
- The plugin binds no typed input: edges are drawn with the cursor. The server accepts any client's typed edge, as the catalogue describes.
- Before both players have moved, the footer with `A abort` is 86 columns and is cut off in a narrower window (the shared footer-wrap entry in `TODO.md`); after that it is 76.
- The clock display assumes the client's and the server's wall clocks agree.
