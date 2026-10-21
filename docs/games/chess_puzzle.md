# Chess puzzles and study

Solo chess along a line you import: find the moves of a puzzle, or guess the moves of a game. Server: `server/games/chesspuzzle/`; board UI: `lua/games/games/chess_puzzle.lua`; puzzle sets: `lua/games/core/puzzles.lua`.

## Rules as implemented
- One seat, `solver`. The game is a start position (`fen`) and a line (`moves`, both sides), and you play one side (`player`).
- **The one rule:** the server plays every move of the line that is not yours, at the start and after each move of yours that it accepts. A Lichess row needs no special case: its first move is the opponent's, played before you are asked. Each of these moves is a `replied` event, those at the start in the game's first update (seq 0).
- Your move is checked against the line's move at that point. A move that is not legal is refused (`illegal_action`) and costs nothing.
- **Puzzle mode** (strict):
  - the line's move is accepted;
  - any other move that mates is accepted too and solves the puzzle, as Lichess accepts an alternative mate;
  - any other legal move ends the game, lost, reason `wrong_move`. The board stays as it was; the move is reported in a `missed` event;
  - reaching the end of the line solves it, reason `solved`.
- **Study mode:**
  - a wrong guess is a miss: the `missed` event reports it, `misses` goes up and the position stays;
  - `reveal` (`n`) plays the line's move for you and also counts as a miss;
  - reaching the end of the line completes it, reason `completed`, a win; the final view keeps `misses`.
  - An alternative mate is a miss in study mode: you are guessing the game's move.
- The other side's moves are only ever the line's. There is no engine.

## Rulings
- **Separate game type.** It shares the chess engine (`games/chess`: FEN, move generation, SAN/UCI parsing, `Play`) but not chess's rules: no opponent, no draws, no clock.
- **Validation at the start.** `game.start` replays the whole line once and refuses, with `bad_request`:
  - a FEN that does not parse (`fen: want 6 fields, got 2`);
  - a line of 0 or more than 600 plies;
  - a move that does not replay, naming its 1-based ply (`ply 3: illegal move: Nf6`), including any move after a mate;
  - a line with no move for `player`.
- The line is normalized once, so moves are compared as moves and the line is never parsed again. SAN, UCI and case are all accepted, as in chess.
- `fen` empty means the standard start position.
- **Not secret.** The line is in the game's options, which the client chose and the server echoes, so it is not hidden; the board never draws `options.moves`. The view's `solution` (the whole line in SAN) is empty until the game is over, however it ends.
- `legal_moves` (UCI) is in the view only while the game runs; giving up, aborting and every ending clear it.
- Every event is `by` the solver's seat, the server's replies included, so none notifies you as if from someone else.

## Puzzle sets (client side)
The server knows nothing of sets; they are the client's, in `stdpath("data")/games/puzzles/<set>.json` as `{entries: [{id, mode, fen, moves, player}], next}`, `next` 1-based.

- `:Games import <file> [set] [max]` reads a `.csv` or `.pgn` file into the set, by default named after the file (letters, digits, `_` and `-` only). It replaces a set of that name and becomes the latest set. It needs no server.
  - The file is read a line at a time, so Neovim stays responsive on a large file, such as the whole Lichess database; a notice reports progress every 100000 lines.
  - The set keeps the file's first `max` entries, by default `puzzles.max_entries` from setup (5000). Reading stops at the first entry past it, so a large file is only read that far; the summary then says `kept N, more entries left unread`, and reports only the failures before that point.
  - **CSV**, the Lichess puzzle database: a header with `PuzzleId`, `FEN` and `Moves` (other columns are ignored); each row is a puzzle for the side **not** to move in its FEN.
  - **PGN**, split into games. Comments (`{}`, which may span lines, and `;` to the end of the line), variations (nested), `%` escape lines, NAGs, move numbers, annotations (`!?`) and the result are stripped. A tag line inside a comment does not start a game.
    - A game with a `FEN` header is a puzzle for the side to move in it.
    - A game without one is a study from the standard start; one prompt at import asks which side to study in all of them.
  - An entry that cannot be read (no moves, a FEN without a side to move, an unbalanced comment) is reported with its line or game number, and the rest are imported. A file with none writes no set.
  - Lines are only split, not checked: the server checks each entry when it starts.
- `:Games puzzle [set]` starts the set's next entry, by default in the latest imported set, opens its board and moves the set past it. An entry the server refuses is reported and skipped. When the set is done it says so; restart it from the finder or import the file again. The set argument completes from the saved sets.
- The set names in `latest` and the set files are checked like an imported name, so no set is ever read or written outside the sets' directory; a `latest` naming no set is ignored with a warning.
- `:Games find puzzles` lists the saved sets with their size and next entry. Its default action starts the set's next entry; `restart`, once confirmed, sends the set back to its first entry.

## Other endings (core)
- Give up (`R`, confirms): a loss with no winner, reason `resign`; the view then shows the solution.
- Abort (`A`): before your first move; result `aborted`, scores nothing.

## Options
All four are sent by `:Games puzzle` from the set's entry. `:Games solo chess_puzzle` asks for them one by one instead.

| Option | Values | Default | Meaning |
|---|---|---|---|
| `mode` | `puzzle`, `study` | `puzzle` | whether a wrong move ends the game or counts as a miss |
| `fen` | text, at most 100 bytes | `""` (the start position) | the start position |
| `moves` | text, at most 4799 bytes: 600 plies of the widest SAN, `Qh4xe1+` | `""` (refused) | the line, both sides, space-separated SAN or UCI |
| `player` | `white`, `black` | `white` | the side you play |

## Controls and keys
Buffer-local, normal mode, in the board buffer.

| Key | Action |
|---|---|
| `m` | enter a move: SAN or UCI, any case (`Qxf7`, `qxf7`, `h5f7`) |
| `n` | reveal the line's move, study mode only; counts as a miss |
| `F` | flip the board |
| `R` | give up |
| `A` | abort, before your first move |
| `]g` | next game waiting on your move |
| `q` | close |

The board is chess's: the same asset pack, legend and move list, facing your side. The move list numbers on from the FEN's move number, so a line from a black-to-move FEN reads `12. ... Bxg3`, `13. Rxe7 …`. Beside it a status panel shows the mode, your side, your moves left in the line and your misses. Once the game is over the panel lists the solution, numbered the same way.

## Errors
| Case | Message |
|---|---|
| a move that is not legal | `illegal move: <text>` |
| a SAN that fits two moves | `ambiguous move: <text> (Bxc4 or bxc4)` |
| reveal in puzzle mode | `reveal is for study mode only` |
| unknown action | `unknown action: <kind>` |
| the game is over | `not your turn` |

## Scoring and result
Solved or completed +1; wrong move or gave up -1; abort 0. Solo games count in their own records but not in the all-games leaderboard. There are no puzzle ratings or statistics.

## Known limits
- A study takes one side for every game of a PGN file; split the file to study different sides.
- A set holds at most `max` entries (5000 by default), since `:Games puzzle` rewrites the whole set file as it moves past each entry. To play further into a larger file, split it, or raise `max` or `puzzles.max_entries`.
- Two Neovims playing the same set can both start its next entry.
