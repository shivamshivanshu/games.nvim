# Poker

No-Limit Texas Hold'em, a sit-and-go for 2 to 6 players, played until one player holds every chip. Server: `server/games/poker/`; table UI: `lua/games/games/poker.lua`.

## Rules as implemented
- **Constants (no options, no clock).** Every player starts with 1000 chips. Blinds are 10/20 and double at the start of hands 11, 21, 31 and so on (20/40, 40/80, ...).
- **Seats.** The challenger is `p1`; invitees follow clockwise as `p2`, `p3`, ... in invite order. The first button is drawn at random.
- **Blinds.**
  - With 3 or more players, the small blind sits left of the button and the big blind next to it. Preflop, action starts left of the big blind. Postflop, it starts at the first live seat left of the button.
  - Heads-up, the button posts the small blind. It acts first preflop and last postflop.
  - A stack shorter than its blind posts all-in for what it has.
  - The button moves to the next seat that still has chips. There are no dead blinds and no missed-blind debts.
- **Moves.** Amounts are **totals for the street** ("raise to"), not increments.
  - `fold`: always allowed on your turn, even when you could check.
  - `check`: only with nothing to call. That includes the big blind's option after limps.
  - `call`: put in what you owe. With nothing to call, `call` plays as `check`.
  - `bet N`: only when no bet is open on the street. N is at least the big blind.
  - `raise N`: only over an open bet. N is at least the current bet plus the last full raise increment. The increment starts at the big blind on every street; preflop, the big blind counts as the opening bet.
  - `allin`: your whole stack. Below what you owe, it is a call for less. Above the bet but short of a full raise, it is an incomplete raise: it does not change the increment and does not reopen betting.
  - A `bet` or `raise` larger than your stack is rejected; use `allin`. A `bet` or `raise` below the minimum is rejected with the minimum.
- **Reopening.** A seat that has already acted on the street may raise again only once the raises made since it acted add up to a full raise. Until then it may only call or fold. Nobody may raise when every other live seat is all-in.
- **What you owe** never exceeds what an opponent can still put in this street. So a short all-in big blind is matched only up to its size by the last seat with chips.
- **Pots.**
  - Each all-in level caps a pot layer.
  - Folded and departed players' chips stay in the layers they reached, but those players cannot win them.
  - A bet no one called is returned to its owner (event `returned`). The exception is a player who has left the table, whose uncalled chips stay in the pot.
  - Pots are awarded from the last side pot down to the main pot, each to the best eligible hand or hands.
  - A split pot is shared equally. Odd chips go one at a time to the tied winners, clockwise from the left of the button.
- **Showdown.**
  - When betting ends with two or more players in, every live hand is shown to everyone; there is no muck.
  - When no more betting is possible because players are all-in, the hands are shown and the board is run out within the same step.
  - A hand won because everyone else folded shows nothing.
- **Hand ranking.** The best 5 of your 2 hole cards and the 5 board cards, over all 21 combinations. From low to high: high card, pair, two pair, trips, straight, flush, full house, quads, straight flush. A-2-3-4-5 (the wheel) is the five-high straight, and straights do not wrap round (Q-K-A-2-3 is not one). Hands compare by category, then by the ranks that make up the category, then by kickers. Suits never break ties.
- **Busting and leaving.**
  - A player with 0 chips after a hand is out (event `busted`).
  - Resigning (`R`) leaves the table (event `left`). A player who leaves mid-hand folds. The chips they already put in, including any uncalled excess, stay in the pot and can be won; the rest of their stack leaves play.
  - The game ends as soon as one player holds every chip in play.
- **Dealing.** The game draws a sub-seed when it starts. Each hand's deck is shuffled from (sub-seed, hand number), so a restored game deals the same cards.

## Rulings and assumptions
1. **The view shows the last hand, not per-seat shown cards.** §3.7 sketched a per-seat `shown_cards`. The view instead carries `last_hand {hand_no, board, shown, awards}`, so hands shown at a showdown stay visible, with the pots they won, until the next hand ends.
2. **Reopening is cumulative (TDA rule 47).** A single incomplete all-in never reopens betting for a seat that has already acted. Several incomplete raises that together add up to a full raise do.
3. **Scores count the whole table.** The rest of a leaver's stack leaves play, so literal chip deltas would not sum to 0. A heads-up winner against a bigger stack that resigns would even score negative. The winner therefore scores `+1000 × (players − 1)` and every other player `−1000`. In a game played out to the end, this equals the chip delta.
4. **A short all-in big blind.** Players must still call the full big-blind level, but what a seat owes is capped by what any opponent can still put in (TDA).
5. **Secrecy.** Hole cards appear only in their owner's view until a showdown shows them. Events never carry unshown hole cards, and the deck order and the sub-seed never reach any client. Because the same card string comes back in later hands, the conformance check judges privacy card by card, against the state both before and after each step.
6. **A leaver's chips.** When a player leaves, every chip they already put in stays in the pot, including an uncalled raise, and goes to whoever wins it. The chips behind leave play.
7. **Folding is always legal on your turn**, even with nothing to call.
8. **`call` with nothing to call is a check**, and is reported as `check`. The `c` key relies on this.
9. **One action kind.** Every move is `{kind: "act", text}`. The text is matched case-insensitively against `fold`, `check`, `call`, `bet N`, `raise N` and `allin`, and the action catalogue publishes that pattern.
10. **An all-in hand that runs out at once.** When all but one player are all-in from the blinds, the hand is played out within the step that dealt it. A single step's events can then cover several hands.
11. **Legality comes from the server.** Only the seat to act gets these fields:
    - `legal`, its allowed moves;
    - `to_call`;
    - `min_raise_to` and `max_raise_to`, the bounds of N for `bet` or `raise`. They are set only when one of those moves is legal, and `max_raise_to` is the all-in total.

    Every other viewer gets `[]` and zeros. The client shows these numbers and does no betting arithmetic of its own. A finished or aborted game offers no moves.

## Controls and keys
Buffer-local, normal mode, in the board buffer. Only the keys valid now are mapped.

| Key | Action |
|---|---|
| `f` | fold, on your turn |
| `c` | check or call, on your turn (sends `call`; with nothing to call it checks) |
| `m` | type a move: `bet 200`, `raise 400`, `allin`, or any other move (prompt `Bet/raise/allin: `) |
| `R` | resign: leave the table (confirms) |
| `A` | abort, while the core offers it |
| `]g` | next game waiting on your move |
| `q` | close |

All-in is only on `m`, so it cannot be hit by accident.

## Display
- **Header (the seats table).** One line per player, in seat order: user, seat, `D` on the button, stack, this street's bet, and `folded`, `all-in` or `out`. The seat to act is marked `← to move`, or `← your move` when it is you.
- **Board.** The five community cards on the top row (`··` for one not yet dealt), with your hole cards below them. Cards show as `A♠ K♥`, or `As Kh` in ascii mode. Under `ambiwidth=double` each unicode card that would widen falls back to its ascii form. On bigger windows the cards grow, and empty board slots are outlined.
- **Panel.** The hand number and blinds, then the settled pots from earlier streets (`60 + side 20`; this street's bets are in the header). On your turn it adds what you owe, the bet or raise range, and your legal moves. After a hand ends it shows the last hand: the hands shown and the pots won.

## Options
None. The stack, blinds and schedule are fixed, and there is no clock.

## Result and scoring
- The result reason is `all_chips` when one player has won every chip. When the last opponent resigns, the core reason is `resign`. An abort is `aborted`, with no winner and no score.
- Scores are chip deltas: the winner gets `+1000 × (players − 1)` and everyone else `−1000`, so they sum to 0. Imposed endings (abort) record no scores.
- There are no exports.

## Known limits
- No per-action clock, so a player who never acts stalls the table until they resign or the game is aborted.
- No configurable stack or blinds, and no hand-history panel beyond the last hand.
- Hearts and diamonds are not coloured red. Telling suits apart relies on the symbol or the letter.
- The panel sits beside the board. On narrow windows the board stays one line per card and the panel can be taller than the board.
- Chips that leave with a player leave play, so the winner never holds `1000 × players`.
