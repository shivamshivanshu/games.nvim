# Battleship

Two players, classic Battleship on 10 × 10. Server: `server/games/battleship/`, with cell names from `server/game/grid/coords.go`; board UI: `lua/games/games/battleship.lua`, drawn with the side-by-side grids of `lua/games/ui/widgets/board.lua` and the cursor of `lua/games/ui/cursor.lua`.

## Rules as implemented
- Each player has a 10 × 10 board. Columns are lettered `a` to `j` from the left, rows numbered `1` to `10` from the top.
- Each fleet is, in placing order: carrier 5, battleship 4, cruiser 3, submarine 3, destroyer 2.
- **Setup.** Both players place at once, each seeing only their own board. A ship lies across (its left end on the chosen cell) or down (its top end there). It must lie wholly on the board and may not overlap another of the player's ships; ships may touch, side by side or end to end.
- **Random fill** places the rest of a player's fleet at once: each ship goes on one of its legal cells and orientations, drawn from the server's seeded random source.
- A placed ship stays where it is; there is no undo.
- When a fleet is complete the server sends `ready`; the other player is told only that the fleet is ready. Each placement still advances the game's `seq` for both players, as every accepted action does, so the opponent can count placements, never see where they went.
- **Battle.** Once both fleets are complete, the players take turns firing one shot each, `p1` first. A shot names a cell of the opponent's board not yet fired at, and reports a miss, a hit, or a sunk ship, naming the ship sunk; a hit never says which ship.
- The turn passes after every shot, hit or miss.
- Sinking the opponent's last ship wins.

## Rulings
| Question | Ruling | Why |
|---|---|---|
| Board and fleet | 10 × 10, ships of 5, 4, 3, 3 and 2 | user ruling, spec §3.9 |
| Placement | by hand, a ship at a time in a fixed order, with a cursor and a rotate key; random fill for the rest | user ruling; a fixed order means the view names the next ship and the client never chooses one |
| Touching ships | allowed | the ruling sets no spacing rule, so none is enforced |
| Placement is private | the opponent sees only that a fleet is ready, never a square of it, until the game is over | hidden information is the game |
| Shooting | one shot a turn, turns alternate whatever the shot found; no salvo | user ruling, "classic shooting" |
| What a shot reveals | miss, hit, or sunk naming the ship | the classic call: "you sank my battleship" |
| Legality on the wire | the view lists the anchors of the next ship, both ways, and the cells the shooter may fire at | the client draws legality and does no fit or overlap arithmetic of its own |
| First shot | `p1`, picked with the shared `first` option | as in the other two-player games |
| Game over | both fleets are shown: each target shows the opponent's ships that were never hit | the game no longer hides anything |
| Cell input | text such as `c5`, column letter then row number, either case, spaces around it ignored | the action catalogue takes text fields only; its pattern is the one the server parses with |

## Other endings (core)
- **Resign** (`R`, confirms): the opponent wins, reason `resign`.
- **Abort** (`A`): available until both players have acted once; result `aborted`.
- There is no draw offer and no clock: placing is simultaneous, which a per-turn clock does not fit.
- After any of these the views offer no placement and no target, and show both fleets, because battleship implements `game.Concluder`.

## Options
| Option | Values | Default | Meaning |
|---|---|---|---|
| `first` | `random`, `me`, `opponent` | `random` | who is `p1` and fires first; `me` is the challenger |

## Controls and keys
| Key | Action |
|---|---|
| `h` `j` `k` `l` | move the cursor left, down, up or right: over your fleet while placing, over the target while firing |
| `r` | turn the ship you place between across and down |
| `<CR>` | place the ship at the cursor, or fire at the cursor's cell when the view offers it |
| `f` | place the rest of your fleet at random |
| `m` | fire at a typed cell, e.g. `c5` |
| `R` | resign |
| `A` | abort, until both players have acted |
| `]g` | next game waiting on your move |
| `q` | close |

The cursor and its orientation belong to the board in your Neovim, not to the game: moving it sends nothing.

## Display
- Two boards side by side, titled `your fleet` and `target`, with lettered columns and numbered rows; a panel beside them.
- Unicode glyphs: water `·`, ship `■`, miss `○`, hit `✕`, sunk `⊠`; ascii: `.`, `#`, `o`, `X`, `*`.
- **Placing:** the next ship is drawn at the cursor as `□` (ascii `+`, tinted `GamesMovable`) where the view says it fits, and as `▧` (ascii `!`, tinted `GamesCheck`) where it does not. The panel names the ship, its orientation and the cell, and adds `blocked` when it does not fit.
- **Firing:** the cursor shows `◎` (ascii `@`, tinted `GamesMovable`) on a cell the view lists as a target, and `▧` (ascii `!`, tinted `GamesCheck`) on any other, such as one already fired at, where `<CR>` sends nothing. The panel names the cell, adding `fired` when no shot may go there, so the aim never rests on colour alone.
- The latest shot is tinted `GamesLastMove` on the board it landed on and named in the panel (`last: their shot at c5`, then `hit`).
- The panel lists every ship with its state in your fleet and theirs: `unplaced`, `afloat` or `sunk`.
- Events notify as `bob's fleet is ready` and `alice fired at c5: sank the destroyer`.
- At one line a cell the boards and panel fit 78 columns; larger windows draw block-art cells, and under `ambiwidth=double` the one-line ascii glyphs stand in.

## Wire
- **Actions:** `{"kind": "place_across", "cell": "a1"}`, `{"kind": "place_down", "cell": "a1"}`, `{"kind": "random"}`, `{"kind": "fire", "cell": "c5"}`.
- **View:** `{phase, own, target, fleet, enemy, next, anchors, targets, last}`. See `docs/protocol.md` and `docs/schema/games/battleship.json`.
- **Events:** `ready {by}`; `shot {by, cell, outcome, ship?}`.
- **Rejections:**
  - `cell must be a1 to j10, a letter then a number, got "<text>"`;
  - `the <ship> <across|down> from <cell> runs off the board`;
  - `the <ship> <across|down> from <cell> overlaps your <ship>`;
  - a shot during setup: `fire once both fleets are placed`;
  - a placement during the battle: `the fleets are placed; fire at a cell`;
  - `you have already fired at <cell>`;
  - an action of any other kind: `unknown action: <kind>`;
  - out of turn, or a placement once your own fleet is complete: `not_your_turn`.

## Result and scoring
- **Reasons:** `fleet_sunk`, a win; plus the core's `resign` and `aborted`.
- **Scores:** win +1, loss −1. No exports.

## Known limits
- No undo of a placed ship, and no re-roll of a random fleet once it is complete.
- The cursor moves with `h` `j` `k` `l` only, not the arrow keys. It stays where it was left, so the first aim is where the last ship was placed.
- The footer is one row, so its labels are kept short (`r turn`, `<CR> place`, `f fill`) to fit 80 columns while abort is still offered.
- The view lists every legal anchor and target cell, up to about 130 squares an update while placing and 100 while firing, so updates are a few kilobytes and the conformance transcript is large.
