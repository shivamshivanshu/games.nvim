# Liar's Dice

Perudo for 2 to 6 players: every player hides five dice in a cup, the table bids on how many of a face lie under all the cups, and a challenge (`dudo`) costs someone a die. The last player with dice wins. Server: `server/games/liarsdice/`; board UI: `lua/games/games/liars_dice.lua`.

## Rules as implemented
- **Seats** are `p1` to `p6`: the challenger is `p1`, then the invitees in invite order. Turns go in seat order, skipping seats with no dice.
- **Cups.** Every player starts with 5 dice. At the start of each round every cup is rolled; a player sees only their own dice.
- **Bids.** A bid `QxF` says there are at least Q dice showing face F under all the cups. `p1` opens round 1; later rounds are opened by the player who lost the last die, or the next seat with dice if that was their last.
- **Ones are wild.** They count toward a bid on any face. A bid on ones counts only the ones.
- **Raising.** Each bid must raise the standing one:
  - a higher face with at least the same quantity, or the same face with a higher quantity;
  - switching to ones needs at least half the quantity, rounded up (`5x4` → `3x1`);
  - switching back from ones needs at least twice the quantity plus one (`3x1` → `7x4`);
  - from ones to ones, a higher quantity.
- **The opening bid may not be on ones.** No bid may name more dice than are in play.
- **Dudo.** Instead of raising, the player to act may challenge the standing bid. Every cup is shown and the dice counting toward the bid are counted. Fewer than bid: the bidder loses a die. As many or more: the challenger loses a die. Every cup is then rerolled and a new round starts.
- **End.** A player who loses their last die is out. The last player with dice wins, reason `last_with_dice`.
- **Dice** come from the game's per-step random stream (seed and step number), so a restored game rolls the same cups.

## Rulings
1. **No palifico.** The round after a player drops to one die is played like any other: ones stay wild and faces may change.
2. **No calza.** Only `dudo` challenges a bid; there is no "exactly right" call that wins a die back. It would add a third outcome to every challenge and a second way to gain dice; left out to keep the rules short.
3. **No opening on ones.** A common Perudo rule, and it keeps the ones' halving rule from opening at a fraction.
4. **Quantities are capped at the dice in play,** so every bid could be true, and the raise minimums the view lists never exceed it.
5. **A player who leaves** (resign or timeout) takes their dice off the table. When one player with dice remains, they win. Otherwise the round's bidding is voided and starts over on the same dice: no reroll, opened by the seat to act, or the next seat with dice when the leaver was to act.
6. **Secrecy.** A player's dice reach only their own view (`my_dice`) until a `dudo` shows every cup. Bid events carry no dice. A test checks it by changing one player's dice and requiring every other player's views and events to stay byte for byte the same.
7. **An imposed ending shows nothing.** An abort or a last-player resign ends the game without showing the cups.
8. **The server computes legality.** For the seat to act, the view lists the least quantity it may bid on each face (`min_bids`) and whether it may call `dudo`; the Lua board shows them and does no bid arithmetic.
9. **No options and no clock.** Abort is the core's: allowed until every player has acted.

## Controls and keys
Buffer-local, normal mode, in the board buffer. Only the keys valid now are mapped.

| Key | Action |
|---|---|
| `m` | bid: type `QxF`, e.g. `3x5` (prompt `Bid, e.g. 3x5: `) |
| `d` | call dudo on the standing bid, on your turn |
| `R` | resign: leave the table (confirms) |
| `A` | abort, until every player has acted |
| `]g` | next game waiting on your move |
| `q` | close |

## Display
- **Header.** One line per player: user, seat, and `N dice` (`1 die`), or `out`. The seat to act is marked `← to move`, or `← your move`.
- **Your cup.** Your dice in a row, `⚀`–`⚅` (ascii `1`–`6`), `·` (ascii `.`) for each die lost. On bigger windows each die is framed, and drawn with pips on the biggest.
- **The table.** `Round N, M dice in play`; the standing bid, `Bid 3 x ⚃ by bob`, or `none yet`; on your turn the least bid on each face, `Raise 2x⚀ 4x⚁ ...` (`Open` before the first bid).
- **The last reveal.** Who called dudo on which bid, how many dice counted and who lost a die, then every cup as it was shown.

**Wire:** `{"kind":"bid","bid":"3x5"}` and `{"kind":"dudo"}`.
- `bid` is text, matched by `^\s*([0-9]{1,2})\s*[xX]\s*([1-6])\s*$`; the action catalogue publishes the pattern.
- The view `{round, seats, bid, total_dice, my_dice, min_bids, can_dudo, last_reveal}`:
  - `seats` is every seat's `{seat, dice, out}`, the number of dice it holds;
  - `bid` is the standing `{by, quantity, face}`, or `null` before the round's first bid;
  - `my_dice` is the viewer's own dice, sorted; `[]` for a spectator or a player out;
  - `min_bids` is the viewer's own `[{face, quantity}]`, `[]` unless it is to act; a face that cannot be bid is absent;
  - `last_reveal` is the last challenge `{round, bid, challenger, counted, loser, cups: [{seat, dice}]}`, or `null` before the first.
- Events: `bid {by, quantity, face}`, `challenged {by, bid, counted, loser, cups}`, and `out {seat}` when a player loses their last die.

## Result and scoring
- Reasons: `last_with_dice`, plus the core's `resign`, `timeout` and `aborted`. There is no draw.
- The winner scores +1 and everyone else −1. An abort scores nothing.

## Errors
| Case | Message |
|---|---|
| not `QxF` | `a bid is quantity x face, e.g. 3x5, got "<text>"` |
| opening on ones | `the opening bid cannot be on ones` |
| too low a raise | `a bid on <F>s needs at least <Q> dice` |
| more dice than in play | `only <N> dice are in play` |
| dudo before any bid | `there is no bid to challenge: open the bidding` |

## Known limits
- Players are not ranked below the winner; the order they went out is not recorded in the result.
- `docs/conformance/liars_dice.ndjson` plays one challenge and a resignation between two players. The cups depend on the server's seed, which a static transcript cannot follow, so it binds the shown cups to placeholders.
