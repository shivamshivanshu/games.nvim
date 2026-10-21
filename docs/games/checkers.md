# Checkers

Two players, American/English checkers (straight checkers) on an 8×8 board. The server code is in `server/games/checkers/` and the board UI in `lua/games/games/checkers.lua`.

## Variant
- **American/English rules (ruling D1).** It is the common English-language variant: short kings, men that capture forward only, and no rule forcing the longest capture.
- Black moves first. Pieces stand on the dark squares only.

## Board and numbering
- The 32 dark squares are numbered 1-32 in standard notation. With White at the bottom, square 1 is b8 and square 32 is g1.
- Row r (1 at the top) holds squares 4(r-1)+1 to 4r, left to right. Odd rows start on column b, even rows on column a.
- Black starts on 1-12 and White on 21-32.
- Square numbers belong to their squares. `F` mirrors only the drawing, and the numbering legend beside the board mirrors with it.

## Rules as implemented
- **Men** move one square diagonally forward, and capture forward only: Black toward 32, White toward 1.
- **Kings** move, and capture, one square diagonally in any direction. There are no flying kings.
- **A capture** jumps an adjacent opponent piece onto the empty square directly beyond it.
- **Captures are mandatory.**
  - When any capture exists, only captures are legal, for every piece of the side to move.
  - The player may choose any capturing piece and any capture path. A shorter chain is legal beside a longer one; there is no majority rule.
- **A multi-jump must be completed.** The move ends only when the piece has no further jump.
- **Crowning.** A man that reaches the far row (row 8 for Black, row 1 for White) becomes a king.
  - Crowning **ends the move**, even in the middle of a jump: the new king does not continue capturing that turn.
- **Jumped pieces** stay on the board until the move ends, and no piece can be jumped twice in one move.
  - A king's chain may return through its starting square, so a path may repeat a square.
  - With short kings, a jumped piece can never stand on a later landing square: landings keep one row parity, and jumped pieces sit on the other. So the only effect of this rule is that no piece is jumped twice.
- **Losing.** The side to move loses when it has no pieces (`no_pieces`) or no legal move (`no_moves`). This is checked after every move.
- **The forty-move draw (ruling).** The game is drawn automatically after 80 plies in a row with no capture and no man move (`forty_move`).
  - A crowning is a man move, so it resets the count.
  - On the 80th quiet ply, a loss for the side to move takes precedence over the draw.
- **Not implemented:** threefold repetition (deferred).

## Move input
- One action is one complete move: `{kind: "move", text}`.
- **A step** is two squares joined by `-`, e.g. `11-15`.
- **A capture** lists every landing square joined by `x`, e.g. `15x24` or `15x24x31`.
- **Matching:** the server trims surrounding spaces, tabs and newlines (exactly the regex's `\s`), then the text must equal a legal move. The catalogue pattern accepts squares 1-32 only, with no leading zeros, and the server rejects the same texts with the same regex. The client sends the text raw.
- **Errors** (`illegal_action`):

  | Message | When |
  |---|---|
  | `illegal move "<text>": write squares 1-32 joined by - for a step or x for each jump, e.g. 11-15 or 15x24x31` | not move notation, e.g. `9`, `e4`, `01-05`, `99-98` |
  | `illegal move <text>: the jump must continue, as in 9x18x27` | the text stops a chain early |
  | `illegal move <text>: a capture is mandatory: 9x18, ...` | a capture exists and the text is a step; the message lists every capture |
  | `illegal move <text>: the captures are 9x18, ...` | a capture exists and the text is a different capture |
  | `illegal move: <text>` | any other move that is not legal |

- Moving out of turn gives `not_your_turn`.

## Options
| Option | Values | Default | Meaning |
|---|---|---|---|
| `time_control` | `none`, `3+2`, `5+0`, `10+5`, `15+10` | `10+5` | minutes + increment seconds |
| `color` | `random`, `black`, `white` | `random` | the challenger's colour; Black moves first |

## Controls and keys
The keys are buffer-local, in Normal mode, in the board buffer. Only keys valid in the current state are mapped.

| Key | Action |
|---|---|
| `m` | enter a move (prompt `Move: `) |
| `F` | flip the board (local only) |
| `R` | resign |
| `D` | offer draw (or accept the opponent's) |
| `X` | decline a pending draw offer |
| `A` | abort, until both have moved |
| `]g` | next game waiting on your move |
| `q` | close |

**Display:**
- The board is drawn from the viewer's side, Black at the bottom if you are Black; `F` toggles it.
- Glyphs are hollow ⛀ ⛁ for White men and kings, and solid ⛂ ⛃ for Black. Under `glyphs = "ascii"` they are `w W b B`. Larger cells draw disc art, and a king's disc carries a ring.
- The squares of the last move's path are tinted `GamesLastMove`.
- On your move, every piece you may move is tinted `GamesMovable`, from the server's legal moves. When a capture is mandatory, only the capturing pieces are tinted.
- Beside the board are the square-numbering legend, the notation key (`-` move, `x` jump) and a two-column numbered move list.

## Wire
- **View:** `{board[32], legal[][], last[], history[]}`.
  - `board[i]` is square i+1: `""` for empty, `b`/`B` for Black's men and kings, `w`/`W` for White's.
  - `legal` lists the viewer's own legal moves as square paths, origin first. It is empty unless the viewer is to move, and empty once the game ends, including an imposed end (`game.Concluder`).
  - `last` is the last move's path, `[]` before the first move.
  - `history` holds the moves in notation.
- **Event:** `moved {by, move, crowned}`.
- **Schema:** `docs/schema/games/checkers.json`. **Transcript:** `docs/conformance/checkers.ndjson`.

## Result and scoring
- **Result reasons:** `no_pieces`, `no_moves` and `forty_move`, plus the core's `resign`, `timeout`, `draw_agreed` and `aborted`.
- **Scoring:** a win is +1, a loss −1 and a draw 0; an abort records nothing. Checkers supplies no score of its own, and has no export.
- **Core endings, the same as chess:**
  - draw by agreement: `D`, accepted when both offer;
  - resign;
  - timeout: a flag always loses;
  - abort, until both players have moved once.

## Known limits
- No threefold-repetition draw: a repeated position plays on until the forty-move rule.
- No takebacks.
- There is no pointer or select-then-move input yet. `legal` is in the view for that.
- Clock display assumes the client's and the server's wall clocks agree.
