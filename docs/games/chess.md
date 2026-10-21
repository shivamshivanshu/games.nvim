# Chess

Two players, standard chess. Server: `server/games/chess/`; board UI: `lua/games/games/chess.lua`.

## Rules as implemented
- Start position is the standard one (`StartFEN`); no Chess960, no custom start.
- Full legal-move generation: castling (rights, empty path, king not in/through check), en passant, promotion to Q/R/B/N.
- Checkmate: the side to move has no legal move and is in check; the other side wins.
- Stalemate: no legal move, not in check; draw.
- Check and mate are marked `+` and `#` in the SAN history.

## Draw rules
| Rule | Trigger | Applied |
|---|---|---|
| Insufficient material | K v K; K+one minor v K; only bishops, all on one square colour, no knights | automatic |
| Fivefold repetition | same position 5 times | automatic |
| Seventy-five-move | halfmove clock >= 150 | automatic |
| Threefold repetition | same position 3 times | claimable |
| Fifty-move | halfmove clock >= 100 | claimable |
| Agreement | one player offers, the other accepts by offering too | core control |

- Automatic endings are checked after every move, in this order: mate, stalemate, insufficient material, fivefold, 75-move. So mate on the move that reaches 150 still wins.
- Position identity for repetition: FEN without counters; the en passant square counts only when a legal en passant capture exists. Castling rights and side to move count.
- Claiming (`claim_draw`): only the side to move may, and only while a threefold or fifty-move condition holds in the current position. Otherwise it is rejected: `no draw can be claimed`. Threefold is reported when both apply.
- Claiming is optional: playing on forgoes nothing; the claim stays available while the position holds.
- Draw by agreement (core, not chess code): either player may offer at any time (not only on their move) with `D`. The draw is imposed when every non-eliminated player has an offer pending. Any move clears the pending offer; `X` (decline) clears it too. Reason `draw_agreed`.

## Other endings (core)
- Resign (`R`, confirms): the opponent wins, reason `resign`.
- Timeout: the player to move whose clock reaches 0 loses, reason `timeout`. There is no insufficient-material exception: a flag always loses.
- Abort (`A`): offered only until both players have moved once; result `aborted`, no winner, scores nothing.

## Clock
- Clock starts only after both players have moved once; before that nothing runs (hence the abort window).
- Increment is added to the mover after each move.
- The server decides timeouts; clients only display a countdown.

## Options
| Option | Values | Default | Meaning |
|---|---|---|---|
| `time_control` | `none`, `3+2`, `5+0`, `10+5`, `15+10` | `10+5` | minutes + increment seconds |
| `color` | `random`, `white`, `black` | `random` | the challenger's colour |

## Controls and keys
Buffer-local, normal mode, in the board buffer; only keys valid in the current state are mapped.

| Key | Action |
|---|---|
| `m` | enter a move (prompt `Move: `) |
| `C` | claim draw, shown only while the server marks a draw claimable for you |
| `F` | flip the board (local only) |
| `R` | resign |
| `D` | offer draw (or accept the opponent's) |
| `X` | decline a pending draw offer |
| `A` | abort, until both have moved |
| `]g` | next game waiting on your move |
| `?` | help: legend and keys |
| `q` | close |

The board is drawn from the viewer's side (black at the bottom if you are black); `F` toggles it. The last move and a checked king are highlighted. The move list is two columns, numbered on from the view's `first_ply`, the plies the start position has behind it (0 for the standard start).

## Move input
- SAN or UCI, any case, e.g. `e4`, `Nf3`, `exd8=Q`, `O-O`, `e2e4`, `e7e8q`.
- Castling accepts `0-0` and `0-0-0`; promotion accepts `e8Q` or `e8=Q`; trailing `+ # ! ?` are ignored.
- Matching: UCI first (against legal moves), then exact SAN, then case-insensitive SAN. If the case-insensitive match is ambiguous the move is rejected (`ambiguous move: ...`); `bxc4` may be a bishop or a b-pawn, so a capital `B` is needed for the bishop.
- Anything else: `illegal move: <input>`.
- Moving out of turn: `not_your_turn`.

## Display: material
- Each player's header entry (after the clock) shows the pieces that side is up, then `+N` if ahead.
- Values: P=1, N=3, B=3, R=5, Q=9; kings are not counted. Per kind, equal counts cancel; only the surplus of the side ahead is shown, never both totals.
- The server computes it from the piece placement and sends it as the view's `material`: per side, `pieces` (the opponent's pieces it is up, most valuable first, as FEN letters) and `score`. Promotions therefore count as the promoted piece.

## Result and scoring
- Result reasons: `checkmate`, `stalemate`, `insufficient_material`, `fivefold_repetition`, `seventy_five_move`, `threefold_repetition`, `fifty_move`, plus core `resign`, `timeout`, `draw_agreed`, `aborted`.
- Win +1, loss -1, draw 0, abort none (no win/loss/draw recorded). Chess supplies no score of its own.
- Leaderboard ranks by most wins, then fewest losses, then most draws, then name.
- Finished games export `game.pgn`: Seven Tag Roster (`Event` "games.nvim", `Site` "?", `Date` UTC start day, `Round` "-", White, Black, Result), `SetUp`/`FEN` only for a non-standard start (never for now), movetext wrapped at 79 columns, result token `1-0`, `0-1`, `1/2-1/2` or `*` (aborted).
- Moves are in SAN with check/mate suffixes and minimal disambiguation.

## Known limits
- No takebacks, no adjournment, no draw offer limits.
- Draw claim is a plain action: no way to claim a draw in advance ("will play X, then claim").
- A flag loses even if the opponent cannot mate.
- Only the starting position is playable, so the PGN `SetUp`/`FEN` tags never appear in practice.
- Clock display assumes client and server wall clocks agree.
- Material is display only, not part of the result.
