# Minesweeper

Solo: open every safe cell of a board of hidden mines without opening a mine. Server: `server/games/minesweeper/`; board UI: `lua/games/games/minesweeper.lua`.

## Rules as implemented
- One seat, `solver`. Start with `:Games solo minesweeper`.
- Every cell starts hidden. A revealed cell shows how many of its 8 neighbours are mines, 0 to 8.
- **Reveal** a hidden cell that has no flag. A 0 floods: every neighbour of a revealed 0 opens too, repeatedly, up to the numbers around the area. A flood never opens a flagged cell.
- **The first reveal always opens a 0.** The board has no mines until then. The first reveal places them from the game's seeded random source, on any cell except the one revealed and its neighbours, so the opening area floods open.
- **Flag** a hidden cell, or take its flag off. A flag guards its cell: a flood stops at it, a reveal on it is refused, and a chord skips it. You may place more flags than there are mines.
- **Chord** a revealed number whose neighbours hold exactly that many flags: every hidden, unflagged neighbour is revealed at once, each flooding as a reveal would. A wrong flag means the chord reveals a mine.
- Revealing a mine, by a reveal or a chord, loses: reason `mine`, no winner.
- Opening every safe cell wins: reason `cleared`. Flags are not needed to win.

## Rulings
- Cells are typed as a row letter, then a column number: `c12` is row c (the third from the top), column 12. Hard has 30 columns, more than the alphabet, so columns are numbers and rows, at most 16, are letters (Battleship style). Either letter case works.
- The mine count follows the classic Windows levels: easy 9×9 with 10 mines, medium 16×16 with 40, hard 16 rows by 30 columns with 99. Easy is the default, as it fits any window.
- The view never holds the place of an unrevealed mine while the game is on. The server keeps the layout, and the conformance tests check that no view or event says `mine` before the end.
- When the game ends, however it ends, the view shows every mine: the one revealed as `exploded`, the others as `mine`, each flag on a mine as `flagged_mine`, and each flag on a safe cell as `wrong_flag`.
- The view carries `chordable`, the revealed numbers a chord can act on now (enough flags around, and something left to open), so the `c` key appears only when a chord is possible. No rule runs in the client.
- No clock and no question-mark marks.

## Other endings (core)
- Give up (`R`, confirms): a loss with no winner, reason `resign`. The final view shows the mines. Giving up before the first reveal shows none, because none are placed yet.
- Abort (`A`): only before your first action; result `aborted`, scores nothing.

## Options
| Option | Values | Default | Meaning |
|---|---|---|---|
| `difficulty` | `easy`, `medium`, `hard` | `easy` | 9×9 with 10 mines, 16×16 with 40, 16×30 with 99 |

## Controls and keys
Buffer-local, normal mode, in the board buffer.

| Key | Action |
|---|---|
| `m` | reveal a cell: type it, e.g. `c12` |
| `f` | flag or unflag a cell |
| `c` | chord a revealed number, when one can be chorded |
| `R` | give up |
| `A` | abort, before your first action |
| `]g` | next game waiting on your move |
| `?` | help: legend and keys |
| `q` | close |

Rows are labelled a-p and columns 1-30. A panel beside the board shows the flags placed and the mines left (mines less flags). Each cell state has its own glyph, so none rests on colour: hidden `░░` (`##` in ascii), a count `1`-`8`, a 0 blank, a flag `⚑` (`F`), and once over a mine `✱` (`*`), a flagged mine `⚑✱` (`F*`), the exploded mine `✸` (`*!`, also tinted) and a wrong flag `✗` (`Fx`). In a roomy window counts and icons are drawn as block art and hidden cells as shaded squares.

## Errors
| Case | Message |
|---|---|
| text not a cell | `want a row letter then a column number, e.g. "c12"; got "<text>"` |
| off this board | `no cell <cell> on a board of R rows by C columns` |
| revealing an open cell | `cell <cell> is already revealed` |
| revealing a flagged cell | `cell <cell> is flagged; take the flag off first` |
| flagging an open cell | `cell <cell> is revealed` |
| chording what is not a revealed number | `cell <cell> is not a revealed number` |
| chording with the wrong flag count | `cell <cell> shows N but has F flags around it` |
| chording with nothing to open | `cell <cell> has no hidden unflagged neighbour` |
| unknown action | `unknown action: <kind>` |
| the game is over | `not your turn` |

## Scoring and result
Cleared +1, lost or gave up -1, abort 0. Solo games count in the minesweeper records but not in the all-games leaderboard.

## Known limits
- Input is typed; there is no cursor to move over the board and click.
- Hard is 91 columns wide at one line per cell, 108 with the side panel, so it overflows an 80-column window.
- The conformance transcript gives up rather than clearing, because the hub picks a random seed; clearing is covered by the Go rules tests and the smoke test.
