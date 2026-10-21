# Chess960

Two players, Fischer random chess. Server: `server/games/chess960/`, on top of `server/games/chess/`; board UI: `lua/games/games/chess960.lua`, which draws with chess's board and asset pack.

A separate game type from `chess`, so its games keep their own stats and leaderboard (spec §3.9).

## Rules as implemented
- Everything in [chess](chess.md) holds: legal moves, check, mate, stalemate, every draw rule and claim, resign, draw offers, abort, the clock and its options.
- Only the start position and castling differ.

## Start position
- Drawn at the start of the game from the game's seeded random source, so a restored game replays from the same position.
- One of the 960 positions: white's back rank has the bishops on opposite-coloured squares and the king somewhere between the two rooks; pawns on the second rank; black mirrors white.
- Numbered 0 to 959 in Scharnagl's numbering, as python-chess numbers them; 518 is the standard array `RNBQKBNR`. The number is in the view, with chess's `material` and `first_ply`, and the board shows `position N`.
- White moves first.
- No option picks the position; every game draws one.

## Castling
- Each side may castle once per rook either side of its king, as in chess: kingside (`O-O`) with the rook on the h-side of the king, queenside (`O-O-O`) with the one on the a-side.
- After castling, the king and rook stand where they would in chess: kingside king on g, rook on f; queenside king on c, rook on d. Either may already stand there; a king on g1 stays put while its rook goes to f1.
- Legal only when:
  - neither the king nor that rook has moved (the right is lost otherwise, or when the rook is captured on its start square);
  - every square the king or the rook crosses or lands on is empty, the two of them aside;
  - the king is not in check, and no square it crosses or lands on is attacked;
  - the king is not in check once both have moved, which rules out a castle whose rook was shielding the king from an attack along the rank.
- Written in UCI as the king's move onto its own rook: `f1h1`, `b1a1`. This is how the server lists it in `legal_moves` and `last_move`, and the board tints those two squares as the last move.
- The FEN is X-FEN: the castling field writes `K`, `Q`, `k` or `q` while the castling rook is the outermost rook of its colour on that side of the king, and otherwise the rook's file (`G` for white's g1 rook once another white rook stands on h1).

## Move input
- As in chess: SAN or UCI, any case.
- Castle with `O-O` / `O-O-O` (or `0-0` / `0-0-0`), or the king onto its own rook (`f1h1`).
- A king's UCI move to the g- or c-file (`e1g1`) is a plain king move, never a castle: with a king on f1 and g1 empty, `f1g1` steps the king and `f1h1` castles.

## Options
| Option | Values | Default | Meaning |
|---|---|---|---|
| `time_control` | `none`, `3+2`, `5+0`, `10+5`, `15+10` | `10+5` | minutes + increment seconds |
| `color` | `random`, `white`, `black` | `random` | the challenger's colour |

## Controls and keys
Chess's: `m` move, `C` claim draw (when the server marks one claimable), `F` flip, `R` resign, `D` offer or accept a draw, `X` decline, `A` abort, `]g` next game, `?` help: legend and keys, `q` close. The board, legend, move list and material display are chess's; a panel beside the move list shows the start position's number.

## Result and scoring
- Result reasons and scoring are chess's: win +1, loss -1, draw 0, abort none.
- `game.pgn` has the Seven Tag Roster, then `[Variant "Chess960"]`, `[SetUp "1"]` and `[FEN "<start>"]`, even from position 518. Castles are `O-O` / `O-O-O` in the movetext.

## Known limits
- Standard chess FENs are the only ones the server reads; a Chess960 FEN cannot seed `chess_puzzle`.
- No choice of start position, and no mirrored or "double" Fischer random variants.
