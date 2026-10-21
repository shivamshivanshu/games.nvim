# Wordle

Solo: find a hidden five-letter word in six guesses. Server: `server/games/wordle/`; board UI: `lua/games/games/wordle.lua`.

## Rules as implemented
- Start it alone with `:Games solo wordle`; the only seat is `solver`.
- The answer is drawn from the game's seed when it starts, from a hand-curated list of about 500 common words.
- Each guess must be a five-letter word from the allowed list: every five-letter word of ENABLE, plus the answers. A guess outside it is refused and costs no guess.
- Each letter of a valid guess gets a mark, computed in two passes:
  1. letters in the right place are `hit`; the answer's other letters are counted;
  2. left to right, a letter that is not a hit is `present` while the answer still has an uncounted copy of it (which it then uses up), otherwise `miss`.

  So a letter never earns more marks than the answer has copies: against `abbey`, `bobby` scores present, miss, hit, miss, hit.
- All five hits solves it: you win, reason `solved`.
- Six valid guesses without a solve: you lose, reason `out_of_guesses`.
- The answer stays hidden from every view and event until the game ends, however it ends; then the view carries it.

## Rulings
- Five letters, six guesses, no options (D11).
- Answers are curated by hand because ENABLE has no frequency data; drawing from all 8636 ENABLE words would give obscure answers. Every answer is also in ENABLE (a test checks this).
- Guesses are case-insensitive and surrounding ASCII whitespace is ignored; only ASCII letters count, so look-alikes such as the Kelvin sign or a non-breaking space are refused.
- Repeating a guess is allowed and uses a guess, as in the original game.
- There is no hard mode: a guess need not reuse earlier hits.
- The server computes all feedback, including the keyboard's per-letter state; the UI scores nothing.
- No clock.

## Other endings (core)
- Give up (`R`, confirms): a loss, reason `resign`; the answer is revealed.
- Abort (`A`): only before the first guess; result `aborted`, no winner, scores nothing. The answer is revealed.
- The state's `Conclude` ends the game, so an imposed ending reveals the answer too.

## Options
None.

## Controls and keys
Buffer-local, normal mode, in the board buffer; only keys valid now are mapped.

| Key | Action |
|---|---|
| `m` | type a guess |
| `R` | give up |
| `A` | abort, before the first guess |
| `]g` | next game waiting on your move |
| `?` | help: legend and keys |
| `q` | close |

The board is six rows of five tiles, one row per guess, empty rows dotted. Beside it a keyboard shows each letter's best mark so far, then the guesses left, or the answer once the game ends.

Feedback never rests on colour alone:

| Mark | Tile colour | One-line glyph | Block art (roomy window) |
|---|---|---|---|
| hit | `GamesHit` | `[A]` | a bar down the right edge |
| present | `GamesPresent` | `(A)` | a dot at the bottom right |
| miss | `GamesMiss` | ` A ` | plain |

## Errors
| Case | Message |
|---|---|
| not five letters a-z | `a guess is 5 letters a-z, got "<text>"` |
| not in the word list | `not in word list: <word>` |
| unknown action | `unknown action: <kind>` |
| the game is over | `not your turn` |

## Scoring and result
Solo scoring: solved +1, failed or given up -1, abort 0. Solo games are left out of the all-games leaderboard; the per-game board keeps them.

## Word lists
`server/games/words/` (shared with Hangman): `allowed.txt` is the five-letter words of ENABLE (public domain), `answers.txt` the curated answers; the README there gives the source URL, checksum and filter.

## Known limits
- One answer list, English only; no daily word or shared seed.
- Block art comes in two sizes (3 and 6 lines); in between, the 3-line letters are centred in larger tiles.
- Under `ambiwidth=double`, the one-line glyphs are used at every size.
