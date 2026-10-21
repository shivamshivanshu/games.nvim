# Hangman

Solo: find a hidden five-letter word, a letter at a time, before six misses hang you. Server: `server/games/hangman/`; board UI: `lua/games/games/hangman.lua`.

## Rules as implemented
- Start it alone with `:Games solo hangman`; the only seat is `solver`.
- The word is drawn from the game's seed when it starts, from Wordle's answer list: about 500 common five-letter words (`server/games/words/`).
- A guess is one letter, or the whole word:
  - a letter in the word reveals every copy of it; a letter not in it is a miss;
  - a whole word must be in Wordle's allowed list (every five-letter word of ENABLE). The right word solves the game at once; a wrong one is one miss and reveals nothing.
- Guessing a letter or a wrong word a second time is refused and costs nothing; so is a word outside the list, or anything that is neither one letter nor five.
- Every letter found, or the word guessed: you win, reason `solved`.
- The sixth miss, of either kind: you lose, reason `hanged`.
- The word stays hidden from every view and event until the game ends, however it ends; then the view carries it. Before then the view shows only the letters found, in place.

## Rulings
- Solo only, six misses, words from Wordle's embedded answer list (spec §3.9).
- Five letters only. The allowed list holds only five-letter words too, so other lengths would need a new list; the answer list also keeps the words common.
- The whole-word guess is in, and a wrong one counts as one miss, as a letter does.
- Guesses are case-insensitive and surrounding ASCII whitespace is ignored; only ASCII letters count. One regular expression, `^\s*([A-Za-z]|[A-Za-z]{5})\s*$`, is both the catalogue pattern and the server's parser.
- The server says whether each guess hit and which letters are found; the UI decides nothing.
- No clock.

## Other endings (core)
- Give up (`R`, confirms): a loss, reason `resign`; the word is revealed.
- Abort (`A`): only before the first guess; result `aborted`, no winner, scores nothing. The word is revealed.
- The state's `Conclude` ends the game, so an imposed ending reveals the word too.

## Options
None.

## Controls and keys
Buffer-local, normal mode, in the board buffer; only keys valid now are mapped.

| Key | Action |
|---|---|
| `m` | type a guess: one letter, or the whole word |
| `R` | give up |
| `A` | abort, before the first guess |
| `]g` | next game waiting on your move |
| `?` | help: legend and keys |
| `q` | close |

The board is the word, one tile per letter. Beside it the gallows gains a body part per miss (head, body, arms, legs), and a panel lists the letters found and missed, each wrong word, the misses left, and the word once the game ends.

Nothing rests on colour alone:

| Tile | Colour | One-line glyph | Block art (roomy window) |
|---|---|---|---|
| found letter | none | ` A ` | the letter |
| hidden letter | none | ` _ ` | a bar along the bottom |
| unfound letter, at the end | `GamesLost` | `(A)` | the letter, with a dot at the bottom right |

The gallows is a picture in the asset pack (`gallows:0` to `gallows:6`), drawn in box-drawing characters, or in plain ascii under `glyphs = "ascii"` or where box drawing would widen (`ambiwidth=double`). The block letters are Wordle's.

## Errors
| Case | Message |
|---|---|
| not one letter or five | `a guess is one letter a-z or a 5-letter word, got "<text>"` |
| a word not in the list | `not in word list: <word>` |
| a letter or wrong word guessed before | `already guessed: <guess>` |
| unknown action | `unknown action: <kind>` |
| the game is over | `not your turn` |

## Scoring and result
Solo scoring: solved +1, hanged or given up -1, abort 0. Solo games are left out of the all-games leaderboard; the per-game board keeps them.

## Known limits
- One answer list, English only, five letters only.
- The gallows has one size; in a roomy window the letters grow and it does not.
