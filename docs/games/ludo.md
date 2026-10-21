# Ludo

Classic Ludo for 2–4 players, ending at the first finisher. The server rules are in `server/games/ludo/`; the board UI is in `lua/games/games/ludo.lua`.

## Seats and board
- Seats are named after the shared seat-colour order **blue, yellow, red, green**. The i-th player takes the i-th seat: the challenger is blue, then the invitees in order. Header colours follow `GamesSeat1`–`4`.
- The track has 52 squares. Clockwise starts are blue 0, red 13, yellow 26 and green 39.
  - With 2 players (blue and yellow), they sit opposite each other.
  - With 3 players, they sit next to each other.
  - This differs from the classic red/green/yellow/blue order, so that seats 1 and 2 face each other.
- **Safe squares:** every start square, plus the star 8 squares past each start (8, 21, 34, 47). The 8 safe squares are the same whatever the seat count.
- **Token progress runs 0–56:**
  - `-1` is in base;
  - `0`–`50` is on the track, at square `(start + progress) mod 52`;
  - `51`–`55` is the seat's own home column;
  - `56` is home.
- The first seat to roll is drawn from the game's seed. Turns then go clockwise.

## Rules as implemented
- `roll` throws one die (1–6). If no token can move, the turn records a pass.
- A 6 earns another roll, even when no token could move.
  - The third 6 in a row **forfeits** the turn. Moves already made with the first two sixes stand.
  - Any other roll ends the run of sixes.
- A token leaves base only on a 6, which puts it on the start square (progress 0).
- Home needs an exact roll. A roll that would overshoot 56 does not move that token.
- You always choose which token to move, even when only one can.
- Captures and reaching home give no bonus roll. Only a 6 does.
- **Stacking and capture:**
  - your own tokens may share any square;
  - tokens of different colours may share safe squares;
  - landing on a non-safe square that holds one opponent token sends it back to base;
  - landing on a non-safe square that holds 2 or more tokens of one opponent colour is illegal, and that token is not movable;
  - pairs never block passing.
- **Ruling: pairs on safe squares.** An opponent pair blocks only on non-safe squares; on safe squares everyone coexists. Open question 4 in the spec ("capture both or illegal") only arises where captures happen.
- **End (D12):** the first seat with all 4 tokens home wins, reason `all_home`. Later placings are not played out.
- **Resign or timeout** removes that player and their tokens. If it was their turn, the turn passes on. When only one player remains, that player wins.

## Options
| Option | Values | Default | Meaning |
|---|---|---|---|
| `time_control` | `none`, `3+2`, `5+0`, `10+5`, `15+10` | `10+5` | minutes + increment seconds |

## Controls and keys
These are buffer-local, Normal-mode keys in the board buffer. The game keys follow your own view, which the server sends per seat: `r` is mapped while your `can_roll` is true, and a token key while that token is in your `movable`, so only the seat to act ever has them. The other keys are the core's controls.

| Key | Action |
|---|---|
| `r` | roll the die, when `can_roll` is true |
| `1`–`4` | move that token by your roll, when it is in `movable` |
| `R` | resign |
| `A` | abort, until every player has acted |
| `]g` | next game waiting on your move |
| `q` | close |

**The board** is the classic 15×15 cross with no labels.
- Bases sit in the corners and home columns lead to the centre.
- Each square shows one glyph: a seat's token shape (blue `●`/`B`, yellow `■`/`Y`, red `▲`/`R`, green `◆`/`G`), with a count digit when several of that seat's tokens share it.
- Several colours on one safe square show `※`/`x` and their total count. `9` stands for 9 or more.
- The latest move's square is highlighted until the next roll.
- The side panel shows the last roll, the sixes in a row, the phase, and each of your tokens: `base`, `track n` (n steps from base), `lane n` (up the home column) or `home`. The ones you may move are marked `←`/`<`, a glyph from the asset pack.

**Wire:** `{"kind":"roll"}` and `{"kind":"move","token":"1".."4"}`.
- The token is a string because the action catalogue describes inputs as string fields, here with the values `1`–`4`.
- The view carries:
  - `starts`: each seat's start square;
  - `safe`: the safe squares;
  - `progress`: each token's steps from base, -1 in base to 56 home;
  - `locations`: where each token stands, as `{kind: "base"}`, `{kind: "track", square: 0..51}`, `{kind: "home_column", step: 1..5}` or `{kind: "home"}`;
  - `last_roll`;
  - `sixes_in_row`;
  - `last`: the move the latest action made, as `{seat, token, to}` with `to` a location; `null` after a roll or once its seat is out;
  - `phase` (`roll`, `move` or `finished`);
  - the viewer's own `movable` and `can_roll`.
- The events are `rolled`, `passed`, `forfeited`, `moved` (`token`, plus `from` and `to` as locations) and `captured` (`victim`, `token`). Each carries `by`.

## Result and scoring
- Result reasons are `all_home`, plus the core's `resign`, `timeout` and `aborted`. There is no draw.
- The winner scores +1 and everyone else −1 (D9). An abort scores nothing.

## Known limits
- 2nd and 3rd places are not played out (deferred).
- The Lua board only maps the view onto its drawing. It draws the 52 track squares as a fixed loop around the cross, each corner's base spots, home column and goal, and finds a seat's corner by the loop square its `starts` names. Where tokens stand (`locations`) and which squares are safe (`safe`) come from the server; Lua does no track arithmetic.
- The client cannot predict dice. Rolls come from the server's per-step RNG, derived from the game seed and the step number, so a restored game replays identically.
- `docs/conformance/ludo.ndjson` plays three players to a finish by resignations, not dice. The first seat and the dice depend on the server's random seed, which a static transcript cannot follow.
