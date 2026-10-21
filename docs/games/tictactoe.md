# Tic-Tac-Toe

Two players, three in a row on a 3x3 board. Server: `server/games/tictactoe/`; board UI: `lua/games/games/tictactoe.lua`.

## Rules as implemented
- Seats are `x` and `o`. X always moves first.
- On your move, place your mark on any empty cell.
- Three of your marks in a row, column or diagonal wins, reason `three_in_a_row`.
- A full board with no line is a draw, reason `board_full`.
- The win is checked first, so completing a line on the ninth cell wins, not draws.
- One move can complete two lines at once (always on the ninth cell); both are reported in `winning_lines`.
- No randomness after the start: the only random choice is who moves first, under `first = random`.

## Rulings
- Standard 3x3, three in a row. No larger boards or k-in-a-row variant; that is Connect4's and Gomoku's ground.
- Cells are numbered 1-9 in phone layout (1 top-left, 9 bottom-right), so the keys read like the board: the top row is 1 2 3.
- The cell travels as text (`"5"`), like every catalogued action input; the server accepts exactly `"1"`-`"9"` (no `+5`, no spaces).
- No clock: a game lasts nine moves at most.

## Other endings (core)
- Resign (`R`, confirms): the opponent wins, reason `resign`.
- Abort (`A`): only until both players have moved once; result `aborted`, no winner, scores nothing.
- After an imposed ending the view drops `legal_cells` (the state's `Conclude` ends the game).

## Options
| Option | Values | Default | Meaning |
|---|---|---|---|
| `first` | `random`, `me`, `opponent` | `random` | who moves first, and plays X; `me` is the challenger |

## Controls and keys
Buffer-local, normal mode, in the board buffer; only keys valid now are mapped.

| Key | Action |
|---|---|
| `1`-`9` | place your mark on that cell; mapped only for the empty cells, on your move |
| `R` | resign |
| `A` | abort, until both have moved |
| `]g` | next game waiting on your move |
| `q` | close |

The footer groups the cell keys, e.g. `3,5-9 place`. Empty cells show their own number. Marks take their player's seat colour from the header; the last move and a winning line are tinted. In a roomy window the marks draw as block art.

## Errors
| Case | Message |
|---|---|
| cell not `"1"`-`"9"` | `cell must be 1-9, got "<text>"` |
| cell already taken | `cell <n> is taken` |
| unknown action | `unknown action: <kind>` |
| not your move, or the game is over | `not your turn` |

## Scoring and result
Default two-player scoring: win +1, loss -1, draw 0, abort 0.

## Known limits
- No undo or takeback.
- The art at cell height 2 is coarse; one-line glyphs `✕`/`○` fall back to `X`/`O` under `ambiwidth=double`.
