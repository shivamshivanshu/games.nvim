# Yahtzee

The dice game for 1 to 4 players: 13 rounds, five dice, three rolls a turn, one scorecard box filled each turn. Server: `server/games/yahtzee/`; board UI: `lua/games/games/yahtzee.lua`.

## Rules as implemented
- **Seats** are `p1` to `p4`: the challenger is `p1`, then the invitees in invite order. `p1` starts and turns go in seat order. Played alone (`:Games solo yahtzee`), the only seat is `p1`.
- **A turn.** Up to 3 rolls of the dice not held. The first roll is a move of its own: a turn starts with no dice. Between rolls, `hold` toggles one die (`"1"` to `"5"` exactly, left to right); held dice keep their face, and with all five held there is nothing to roll, so `roll` is refused. After any roll the seat may score; after the third it must. Scoring a box ends the turn and clears the holds.
- **The scorecard.** Every box is filled exactly once, with a score or with 0.

  | Box | Scores |
  |---|---|
  | Ones to Sixes | the sum of the dice showing that face |
  | 3 of a kind | the sum of all dice, with at least 3 alike; else 0 |
  | 4 of a kind | the sum of all dice, with at least 4 alike; else 0 |
  | Full house | 25 for three of one face and two of another; else 0 |
  | Small straight | 30 for four in a run (1-2-3-4, 2-3-4-5 or 3-4-5-6); else 0 |
  | Large straight | 40 for five in a run; else 0 |
  | Yahtzee | 50 for five alike; else 0 |
  | Chance | the sum of all dice |

- **Upper bonus.** 35 once Ones to Sixes total 63 or more.
- **Yahtzee bonus and the forced joker rule** (the official rule). A Yahtzee rolled when the Yahtzee box is already filled is a joker:
  - it earns a 100 bonus if the Yahtzee box holds 50, and no bonus if it holds 0;
  - it must go in the upper box of its face if that box is open;
  - otherwise in any open lower box, where Full house, Small straight and Large straight pay their full 25, 30 and 40, and the others score as usual;
  - otherwise in any open upper box, for 0.
- **End.** The game ends when every card still in play is full, after 13 rounds. The highest total wins, reason `scorecards_full`. Seats tied for the top all win. When every seat ties, the game is a draw; once a player has left, a tie among the rest makes each of them a winner, so the leaver alone loses. A solo game that reaches the end is a win.
- **Dice** come from the game's per-step random stream (seed and step number), so a restored game rolls the same dice.

## Rulings
1. **Forced joker, not free choice.** The official rules force the joker into its upper box first; the free-choice house rule is not offered.
2. **The first roll is explicit.** A turn starts with empty dice, so the seat sees its card before it rolls, and `hold` and `score` are refused until it does.
3. **Holding is not an event.** `hold` changes only the view of the seat to act; other seats see the holds in their view but get no event line.
4. **The server scores everything.** The view carries, for the seat to act, the boxes it may fill now and what each would score (`options`); the Lua board shows those numbers and computes none.
5. **A player who leaves** (resign or timeout) keeps their card, marked `out`, and their turns are skipped.
   - With 3 or 4 players still in, the game goes on; a player out cannot win.
   - When 2 are left, the other wins.
   - Alone, giving up is a loss with no winner.
6. **No options and no clock.** Abort is the core's: allowed until every player has acted.

## Controls and keys
Buffer-local, normal mode, in the board buffer. Only the keys valid now are mapped.

| Key | Action |
|---|---|
| `r` | roll the dice not held, while you have rolls left and hold fewer than five |
| `1`–`5` | hold or release that die, after a roll with a roll left |
| `a`–`f` | score Ones to Sixes |
| `g`–`l` | score 3 of a kind, 4 of a kind, Full house, Small straight, Large straight, Yahtzee |
| `n` | score Chance |
| `R` | resign, or give up alone (confirms) |
| `A` | abort, until every player has acted |
| `]g` | next game waiting on your move |
| `q` | close |

A box's letter shows on the scorecard, and is mapped, only while that box is one you may score now; `m` is skipped because the shell reserves it.

## Display
- **Header.** One line per player: user, seat, `total N`, and `out` once they left. The seat to act is marked `← to move`, or `← your move`.
- **Dice.** Five dice in a row, `⚀`–`⚅` (ascii `1`–`6`), `·` (ascii `.`) before the first roll. A mark under a die shows it held: `▲` (ascii `^`), and a held die is tinted `GamesMovable`. On bigger windows each die is framed, and drawn with pips on the biggest.
- **Scorecard.** One row per box, then Upper, Bonus at 63, Yahtzee bonus and Total; one column per seat, `-` for an open box. On your turn a `now` column shows what each box you may score would get. Below: `Round X of 13, N rolls left`. The bonus threshold and the round count come from the view's `bonus_at` and `rounds`.

**Wire:** `{"kind":"roll"}`, `{"kind":"hold","die":"1".."5"}`, `{"kind":"score","box":"full_house"}`.
- `die` and `box` are strings because the action catalogue describes inputs as string fields with their values.
- The view `{round, rounds, dice, held, rolls_left, cards, can_roll, can_hold, options, bonus_at}`, `rounds` the 13 rounds of a game and `bonus_at` the upper total (63) that earns the upper bonus:
  - `dice` is `[]` before the turn's first roll, then all five faces;
  - `cards` holds every seat's `{seat, scores, upper_total, upper_bonus, yahtzee_bonus, total, out}`, `scores` keyed by box and holding the filled boxes only;
  - `can_roll`, `can_hold` and `options` are the viewer's own: false and `[]` unless it is to act.
- Events: `rolled {by, dice}`, `scored {by, box, score, yahtzee_bonus}`.

## Result and scoring
- Reasons: `scorecards_full`, plus the core's `resign`, `timeout` and `aborted`.
- Scoring is the core's (D9): +1 a win, −1 a loss, 0 a draw; an abort scores nothing. Card totals stay in the view and the final result payload.
- The leaderboard counts only games with more than one player, so solo games never rank.

## Errors
| Case | Message |
|---|---|
| hold or score before rolling | `roll first` |
| roll or hold with no rolls left | `no rolls left: score a box` |
| roll with every die held | `every die is held: release one or score a box` |
| die out of range | `die must be 1 to 5, got "<text>"` |
| unknown box | `no box "<text>"` |
| box already filled | `<box> is already scored` |
| joker into a box the rule forbids | `the joker rule allows only [<boxes>]` |

## Known limits
- `docs/conformance/yahtzee.ndjson` plays solo: one roll, a hold, a second roll, a score and a give-up. The dice depend on the server's seed, which a static transcript cannot follow, so it binds them to placeholders.
