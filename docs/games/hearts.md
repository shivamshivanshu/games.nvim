# Hearts

The trick-taking game for exactly 4 players: pass three cards, follow suit, and avoid taking hearts and the queen of spades. Hands are played until a score reaches 100; the lowest score wins. Server: `server/games/hearts/`; board UI: `lua/games/games/hearts.lua`.

## Rules as implemented
- **Seats** are `p1` to `p4`: the challenger is `p1`, then the invitees in invite order, clockwise. The seat after yours is on your left.
- **The deal.** Each hand deals the 52-card deck, 13 cards each.
- **Passing.** Before play every player chooses three cards to pass: hand 1 to the left, hand 2 to the right, hand 3 across, hand 4 no passing, and so round again. The cards change hands once all four have chosen; a choice cannot be changed.
- **Tricks.** The holder of the 2♣ leads it to the first trick. Each player in turn plays one card, following the suit led if they can; the highest card of the suit led takes the trick, and its taker leads the next.
- **Hearts are broken** when a heart is first played to a trick. Until then a heart may not be led, unless the leader holds only hearts.
- **No points on the first trick.** A player void in clubs may not play a heart or the Q♠ to it, unless they hold nothing else.
- **Points.** Each heart taken is 1 point and the Q♠ 13, 26 in a hand.
- **Shooting the moon.** A player who takes all 26 scores 0 for the hand and everyone else 26.
- **End.** After a hand in which any score reaches 100 or more, the match ends, reason `score_limit`. The lowest score wins; a tie for lowest shares the win, and a tie between all four is a draw.
- **The shuffle** for each hand is seeded from the game's random stream and the hand number, so a restored game deals the same cards.

## Rulings
1. **The Q♠ does not break hearts,** and may be led at any time after the first trick.
2. **A player who leaves** (resign or timeout) is played for until the match ends: it passes its three highest cards, and plays its first legal card in suit-then-rank order (clubs, diamonds, spades, hearts). Its score still counts toward the 100 that ends the match, but it cannot win. When three players have left, the last one wins.
3. **Secrecy.** A hand reaches only its own player's view (`my_hand`). The cards passed reach only the giver (`passed`) and the receiver (`received`); `pass_chosen` events say only who has chosen. Tests swap cards between two hands and require the other two players' views and events to stay byte for byte the same.
4. **The server computes legality.** For the seat to act, the view lists the cards it may play (`legal`) and how many it must still pass (`must_pass`); the Lua board marks them and does no rule checking.
5. **No options and no clock.** The 100-point limit is fixed. Abort is the core's: allowed until every player has acted.

## Controls and keys
Buffer-local, normal mode, in the board buffer. Only the keys valid now are mapped.

| Key | Action |
|---|---|
| `m` | play a card to the trick: rank `2`-`9`, `t`, `j`, `q`, `k`, `a` then suit `c`, `d`, `h`, `s`, e.g. `qs` (prompt `Play, e.g. qs: `) |
| `p` | pass three cards, e.g. `2c qs ah` (prompt `Pass three, e.g. 2c qs ah: `), while you have still to choose |
| `R` | resign: leave the game (confirms) |
| `A` | abort, until every player has acted |
| `]g` | next game waiting on your move |
| `q` | close |

## Display
- **Header.** One line per player: user, seat, score `N pts`, the points taken this hand `+N`, where the seat sits from you (`left`, `across`, `right`), and `passed` once it has chosen its cards, or `out`.
- **The trick.** The cards played to the current trick round the table: you at the bottom, the seat on your left on the left. An empty place is `··` (ascii `..`), framed on bigger windows. A spectator sees `p1` at the bottom.
- **The table.** The hand number and its pass direction; whether hearts are broken; the last trick, who took it and its points; the cards you passed and to whom, and those you received and from whom; what you may do now.
- **Your hand.** Your cards, hearts and diamonds in red, a `^` under each card you may play.
- **The last hand.** What each seat scored in it, and who shot the moon.

**Wire:** `{"kind":"pass","cards":"2c qs ah"}` and `{"kind":"play","card":"qs"}`.
- `cards` is three cards apart, `card` one, matched case-insensitively by `^\s*([2-9TJQKA][CDHS])\s+([2-9TJQKA][CDHS])\s+([2-9TJQKA][CDHS])\s*$` and `^\s*([2-9TJQKA][CDHS])\s*$`; the action catalogue publishes the patterns. Cards on the wire are rank then suit, `Qs`, `Th`, `2c`.
- The view `{hand_no, pass, phase, score_limit, seats, trick, last_trick, hearts_broken, my_hand, legal, must_pass, pass_to, passed, received, last_hand}`:
  - `pass` is `left`, `right`, `across` or `hold`; `phase` is `pass` or `play`;
  - `seats` is every seat's `{seat, score, taken, pass_chosen, out}`, `taken` the points it has taken this hand;
  - `trick` is the trick in play, `[{seat, card}]` in play order; `last_trick` the last one taken, `{plays, winner, points}`, or `null` before the first;
  - `my_hand`, `legal`, `must_pass`, `pass_to`, `passed` and `received` are the viewer's own: `legal` is `[]` unless it is to play, `must_pass` 0 once it has chosen, `pass_to` `null` in a hand without passing;
  - `last_hand` is the last hand's `{hand_no, points: [{seat, points}], moon}`, `moon` the seat that shot it or `null`; `null` before the first hand ends.
- Events: `hand_started {hand_no, pass}`, `pass_chosen {seat}`, `passed {pass}` once the cards change hands, `played {seat, card}`, `trick_won {seat, points}`, `hand_scored {score}`, `left`. An event's `seat` differs from its `by` for a seat played for.

## Result and scoring
- Reasons: `score_limit`, plus the core's `resign`, `timeout` and `aborted`. All four tied is a draw.
- Each winner scores +1 and everyone else −1; a draw scores 0. An abort scores nothing. Match points stay in the view.

## Errors
| Case | Message |
|---|---|
| not a card | `"<text>" is no card: want a rank 2-9, t, j, q, k or a, then a suit c, d, h or s, e.g. qs` |
| not three cards | `"<text>" is not three cards: want three cards apart, e.g. 2c qs ah` |
| a card twice | `"<text>" names a card twice: pass three different cards` |
| a card not held | `you hold no <card>` |
| playing while passing | `you cannot play now: choose three cards to pass <direction>` |
| passing while playing | `you cannot pass now: play a card to the trick` |
| first lead | `the first trick is led with the 2c` |
| leading hearts unbroken | `hearts are not broken: lead another suit` |
| not following | `you must follow <suit>` |
| points on the first trick | `no points on the first trick: play a card that is no heart and not the Qs` |

## Known limits
- Players are not ranked below the winners; the match scores are in the final view, not the result.
- `docs/conformance/hearts.ndjson` deals four players, tries two illegal moves, and ends by resignations. The deal depends on the server's seed, which a static transcript cannot follow, so it binds the cards to placeholders and plays no card.
