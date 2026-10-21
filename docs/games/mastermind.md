# Mastermind

Solo: find a hidden code of four coloured pegs in ten guesses. Server: `server/games/mastermind/`; board UI: `lua/games/games/mastermind.lua`.

## Rules as implemented
- Start it alone with `:Games solo mastermind`; the only seat is `solver`.
- The secret is 4 pegs drawn from the game's seed, each one of 6 colours; colours may repeat.
- The palette is one letter per colour:

  | Letter | Colour | Unicode peg | Ascii peg |
  |---|---|---|---|
  | `r` | red | `Ⓡ` | `R` |
  | `g` | green | `Ⓖ` | `G` |
  | `b` | blue | `Ⓑ` | `B` |
  | `y` | yellow | `Ⓨ` | `Y` |
  | `o` | orange | `Ⓞ` | `O` |
  | `p` | purple | `Ⓟ` | `P` |

- A guess is 4 palette letters, e.g. `rgby`. Each guess is answered with two counts, as the classic game's key pegs show:
  - **black**: pegs of the right colour in the right place;
  - **white**: pegs of the right colour in the wrong place.

  The counts come from the same scoring as Wordle's feedback (`server/game/feedback`): blacks first, then a misplaced colour counts white only while the secret still has an unmatched copy of it. So against `rrgb`, `rrrr` scores 2 black and 0 white, not 2 and 2.
- Four blacks solves it: you win, reason `solved`.
- Ten guesses without a solve: you lose, reason `out_of_guesses`.
- The secret stays hidden from every view and event until the game ends, however it ends; then the view carries it.

## Rulings
- 6 colours, 4 pegs, duplicates allowed, 10 guesses, solo only (spec §3.9).
- Feedback is the count pair, not per peg: the classic game never says which peg earned which key peg.
- Colours are letters on the wire and in input, so a peg is always its letter as well as its colour (C2). Guesses are case-insensitive and surrounding ASCII whitespace is ignored; pegs may not be separated by spaces. One regular expression, `^\s*([rgbyopRGBYOP]{4})\s*$`, is both the catalogue pattern and the server's parser.
- Repeating a guess is allowed and uses a guess.
- The server computes all feedback; the UI scores nothing.
- No clock.

## Other endings (core)
- Give up (`R`, confirms): a loss, reason `resign`; the secret is revealed.
- Abort (`A`): only before the first guess; result `aborted`, no winner, scores nothing. The secret is revealed.
- The state is a `game.Concluder`, so an imposed ending reveals the secret too.

## Options
None.

## Controls and keys
Buffer-local, normal mode, in the board buffer; only keys valid now are mapped.

| Key | Action |
|---|---|
| `m` | type a guess, e.g. `rgby` |
| `R` | give up |
| `A` | abort, before the first guess |
| `]g` | next game waiting on your move |
| `q` | close |

The top row is the secret, `?` per peg until the game ends. Below it are ten rows, one per guess: the 4 pegs, then 4 key pegs, blacks first, then whites, then a dot for each peg that scored neither. Empty rows show holes. The solving row is tinted `GamesWin`. Beside the board: the palette, what the key pegs mean, and the guesses left.

Nothing rests on colour alone:

| Piece | Colour | Unicode | Ascii |
|---|---|---|---|
| peg | `GamesPegRed` … `GamesPegPurple` | its circled letter | its capital letter |
| black key peg | none | `●` | `*` |
| white key peg | none | `○` | `+` |
| no key peg | none | `·` | `.` |
| empty hole | none | `◦` | `.` |
| hidden secret peg | none | `?` | `?` |

## Errors
| Case | Message |
|---|---|
| not 4 palette letters | `a guess is 4 colours from rgbyop, got "<text>"` |
| unknown action | `unknown action: <kind>` |
| the game is over | `not your turn` |

## Scoring and result
Solo scoring: solved +1, failed or given up -1, abort 0. Solo games are left out of the all-games leaderboard; the per-game board keeps them.

## Known limits
- No options: the palette size, code length and guess count are fixed.
- The pack has no block art: in a roomy window the one-line pegs sit in larger cells.
- Under `ambiwidth=double` the circled letters widen, so the ascii pegs are drawn.
