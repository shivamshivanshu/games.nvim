# Sudoku

Solo: fill a 9x9 grid so every row, column and 3x3 box holds 1-9. Server: `server/games/sudoku/`; board UI: `lua/games/games/sudoku.lua`.

## Rules as implemented
- One seat, `solver`. Start with `:Games solo sudoku`.
- The puzzle's givens cannot change.
- Write a digit 1-9 into any other cell, overwrite it, or clear it with 0 or the `clear` action.
- Any digit is accepted, even one that repeats in its row, column or box. The view marks such cells as `conflicts`, so you can see them, but nothing is refused.
- The game is solved, reason `solved`, when the grid equals the solution. The puzzle has exactly one solution, so a full grid with no conflicts is the solution.
- A full grid with a wrong digit plays on until you fix it.

## Puzzle generation
The puzzle is generated when the game starts, from the game's seed, so a restart rebuilds the same puzzle.
1. A complete grid is filled by randomized backtracking: each step picks the empty cell with the fewest candidates and tries those candidates in random order.
2. Every cell is visited once, in random order. Each one is removed, and put back if the puzzle then has 2 or more solutions. The solver stops counting at 2.
3. It stops when the givens reach the target: easy 40, medium 32, hard 26.
4. A puzzle that becomes minimal above the target (no given can go) is kept as it is; the generator never retries. Some hard puzzles end at 27.

Generation takes under a millisecond.

## Rulings
- Difficulty means the givens count only, not the solving techniques needed.
- Input is typed text, `RC D`: row, column, digit. `53 7` writes 7 at row 5, column 3; `53 0` clears it, as does `clear` with `RC`, e.g. `53`. Rows and columns are 1-9 from the top left.
- Wrong digits are not revealed while you play: only repeats visible on the grid itself are marked. The solution reaches the client only when the game is over.
- No clock: a puzzle takes as long as it takes.

## Other endings (core)
- Give up (`R`, confirms): a loss with no winner, reason `resign`.
- Abort (`A`): only before your first move; result `aborted`, scores nothing.
- Once the game is over, by solving, giving up or aborting, the view carries the solution.

## Options
| Option | Values | Default | Meaning |
|---|---|---|---|
| `difficulty` | `easy`, `medium`, `hard` | `medium` | givens kept: 40, 32, 26 |

## Controls and keys
Buffer-local, normal mode, in the board buffer.

| Key | Action |
|---|---|
| `m` | set a cell: type `RC D`, e.g. `53 7`; `53 0` clears it |
| `x` | clear a cell: type `RC`, e.g. `53` |
| `R` | give up |
| `A` | abort, before your first move |
| `]g` | next game waiting on your move |
| `?` | help: legend and keys |
| `q` | close |

Rows and columns are labelled 1-9, and thin lines mark the 3x3 boxes. Givens are muted and bold (`GamesGiven`), your digits take your seat colour, and empty cells show `·` (`.` in ascii). A conflicting digit is framed `[5]` and tinted. After the game, digits from the solution fill the empty and wrong cells as `(5)`, tinted. In a roomy window the digits are drawn as block art, with bars either side of a conflict and corner ticks under a revealed digit.

## Errors
| Case | Message |
|---|---|
| text not `RC D` | `want "RC D", e.g. "53 7" for row 5, column 3, digit 7 (0 clears); got "<text>"` |
| a cleared cell not `RC` | `want "RC", e.g. "53" for row 5, column 3; got "<cell>"` |
| a given | `cell R,C is a given` |
| writing the digit already there | `cell R,C already holds D` |
| clearing an empty cell | `cell R,C is already empty` |
| unknown action | `unknown action: <kind>` |
| the game is over | `not your turn` |

## Scoring and result
Solved +1, gave up -1, abort 0. Solo games count in the sudoku records but not in the all-games leaderboard.

## Known limits
- No pencil marks, hints, undo or cursor input.
- No state rests on colour alone: givens are bold, conflicts framed `[5]` and revealed digits `(5)`. Bold depends on the terminal's font.
- The conformance transcript gives up rather than solving, because the hub picks a random seed; solving is covered by the Go rules tests and the smoke test.
