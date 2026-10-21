# Uno

The shedding card game for 2 to 6 players, by the official rules: match the top card of the discard pile by colour or face, or play a wild, and be the first to empty your hand. Server: `server/games/uno/`; board UI: `lua/games/games/uno.lua`.

## Rules as implemented
- **Seats** are `p1` to `p6`: the challenger is `p1`, then the invitees in invite order, clockwise.
- **The deck** is the 108-card Uno deck: in each of red, yellow, green and blue (`R`, `Y`, `G`, `B`) one 0 and two each of 1-9, Skip (`S`), Reverse (`R`) and Draw Two (`+2`); four Wild (`W`) and four Wild Draw Four (`W+4`). A card is written colour then face, `R5`, `GS`, `B+2`, `W`, `W+4`.
- **The deal.** A random dealer deals 7 cards each, starting left of the dealer, and turns up the next card to start the discard pile. A Wild Draw Four turned up is put back and the deck shuffled until another card shows.
- **The card turned up** acts as if the dealer had played it:
  - a Skip skips the player left of the dealer; a Draw Two makes them draw two and lose their turn;
  - a Reverse turns play counterclockwise, so the dealer plays first;
  - a Wild lets the player left of the dealer name the colour, then play.
- **Your turn.** Play a card matching the colour in play or the top card's face, or any wild; or draw one card. After drawing you play the card drawn, if it matches, or keep it (`k`), which ends your turn. Every draw waits on that choice, matching card or not, so no other player learns whether you could have played it.
- **Action cards.**
  - Skip: the next player loses their turn.
  - Reverse: play turns round. With two players left it acts as a Skip.
  - Draw Two: the next player draws two and loses their turn.
  - Wild: you name the colour in play.
  - Wild Draw Four: you name the colour, and the next player either accepts, drawing four and losing their turn, or challenges.
- **The challenge.** A Wild Draw Four is legal only when its player held no card of the colour in play before it. Challenged and guilty, its player draws four instead and the challenger plays on; innocent, the challenger draws six and loses their turn.
- **The draw pile** runs out into a reshuffle of the discard pile under its top card.
- **End.** The first player to play their last card wins, reason `went_out`.
- **The shuffle** is seeded from the game's random stream, so a restored game deals and reshuffles the same cards.

## Rulings
1. **One hand, no points.** The official game plays hands to 500 points; here the first hand decides.
2. **No stacking.** A Draw Two or Wild Draw Four cannot be answered with another; the player facing one draws.
3. **No "UNO" call.** There is no penalty for not calling it, which needs a race the turn-based core cannot hold. Every hand's size is public, and the header marks a player on their last card `UNO`.
4. **A Wild Draw Four may be played any time,** a bluff included; only a challenge tests it, against the colour in play before it.
5. **The server checks a challenge** without showing the offender's hand to the challenger, unlike the table game. Only the verdict, `guilty`, is public.
6. **The last card ends the game at once.** A Draw Two or Wild Draw Four played as the last card does not make the next player draw.
7. **Drawing when both piles are spent** takes no card and passes the turn.
8. **A player who leaves** (resign or timeout) takes their hand out of play. When one player remains, they win. A Wild Draw Four the leaver played, or faced, is void; if the leaver was to act, the turn passes on.
9. **Secrecy.** A hand reaches only its own player's view (`my_hand`); `drew` events carry the count, not the cards, and a draw goes on the same way whether or not the card drawn can be played. Tests change one hand, or the draw pile, and require every other player's views and events to stay byte for byte the same.
10. **The server computes legality.** For the seat to act, the view lists the action kinds it may take (`legal`) and the cards it may play (`playable`); the Lua board marks them and does no matching.
11. **No options and no clock.** Abort is the core's: allowed until every player has acted.

## Controls and keys
Buffer-local, normal mode, in the board buffer. Only the keys valid now are mapped.

| Key | Action |
|---|---|
| `m` | play a card: type it, e.g. `r5`, `g+2`, `bs`; a wild with the colour it names, `w g` or `w+4 b` (prompt `Play, e.g. r5 or w g: `) |
| `d` | draw a card, on your turn |
| `k` | keep the card you just drew |
| `c` | challenge the Wild Draw Four played on you |
| `a` | accept the Wild Draw Four: draw four |
| `r` `y` `g` `b` | name the colour for a Wild turned up to start |
| `R` | resign: leave the game (confirms) |
| `A` | abort, until every player has acted |
| `]g` | next game waiting on your move |
| `?` | help: legend and keys |
| `q` | close |

## Display
- **Header.** One line per player: user, seat, and `N cards` (`1 card UNO`), or `out`.
- **The pile.** The top card of the discard pile, drawn in the colour in play (a wild in the colour it named), beside the draw pile.
- **The table.** `Colour green (G)`, by name and letter so colour is never the only cue; the direction of play; the draw pile's size; on your turn what you may do now.
- **Your hand.** Your cards, each in its colour, a `^` under each card you may play. Unicode draws Skip as `⊘` and Reverse as `⇄`; ascii writes cards as typed, `RS`, `RR`.

**Wire:** `{"kind":"play","card":"r5"}`, `{"kind":"play","card":"w+4 b"}`, `{"kind":"draw"}`, `{"kind":"pass"}`, `{"kind":"challenge"}`, `{"kind":"accept"}`, `{"kind":"color","color":"G"}`.
- `card` is text, matched case-insensitively by `^\s*(?:([RYGB])([0-9]|S|R|\+2)|(W(?:\+4)?)\s*([RYGB]))\s*$`; the action catalogue publishes the pattern. `color` is one of `R`, `Y`, `G`, `B`.
- The view `{top, color, direction, phase, draw_pile, seats, my_hand, legal, playable, prompt}`:
  - `top` is the discard pile's top card; `color` the colour in play, `""` only while a Wild turned up waits to be named;
  - `direction` is `clockwise` or `counterclockwise`; `phase` is `color`, `play`, `drawn` (the card just drawn may be played, if it matches, or kept) or `challenge`;
  - `seats` is every seat's `{seat, cards, out}`, `cards` the count it holds;
  - `my_hand` is the viewer's own cards; `[]` for a spectator;
  - `legal`, `playable` and `prompt` are the viewer's own, `[]` and `""` unless it is to act;
  - `prompt` is what the seat to act is to do: `color`, `play_or_draw`, `draw` (nothing to play), `play_or_keep` (the card drawn matches), `keep` (it does not) or `challenge`; the board words it and derives nothing.
- Events: `started {dealer, top}`, `played {by, card, color}`, `drew {seat, count}`, `passed`, `skipped {seat}`, `reversed {direction}`, `color {by, color}`, `challenged {by, seat, guilty}`, `reshuffled {cards}`, `left`.

## Result and scoring
- Reasons: `went_out`, plus the core's `resign`, `timeout` and `aborted`. There is no draw.
- The winner scores +1 and everyone else −1. An abort scores nothing.

## Errors
| Case | Message |
|---|---|
| not a card | `"<text>" is no card to play: want a colour letter r, y, g or b and a face ...` |
| a card not held | `you hold no <card>` |
| no match | `<card> does not match the <colour> colour or the <face> on the pile` |
| another card after drawing | `after drawing you may play only the card you drew, <card>` |
| out of phase | `you cannot <kind> now: <what to do>` |
| not a colour | `"<text>" is no colour: want R, Y, G or B` |

## Known limits
- Players are not ranked below the winner, and hands left are not scored.
- `docs/conformance/uno.ndjson` deals three players and ends by resignations. The deal depends on the server's seed, which a static transcript cannot follow, so it binds the cards to placeholders and plays no card.
