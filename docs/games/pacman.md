# Pac-Man

Solo, in real time: steer Pac-Man through the arcade maze, eating every dot while four ghosts hunt it, over three levels. Server: `server/games/pacman/`; board UI: `lua/games/games/pacman.lua`. It is the first game the server ticks on its own (`game.Ticking`; see "Ticks" in `docs/protocol.md`).

## Rules as implemented
- One seat, `player`. Start with `:Games solo pacman`. The game starts paused; the first turn starts it.
- **Maze.** The arcade's 28×31 maze: 240 dots, 4 power pellets, the ghost house with its door, and the side tunnel, whose ends meet.
- **Ticks.** The server moves the game one tick at a time: 150 ms on level 1, 135 ms on level 2 and 120 ms on level 3.
- **Pac-Man** moves one cell a tick, the way it faces, and stops at a wall. A turn is held until the first cell where that way is open, then taken; a newer turn replaces it, and turning back is always open. Pac-Man never enters the ghost house.
- **Eating.** A dot scores 10, a power pellet 50. A pellet frightens every ghost that is not already eyes, for 6, 4 and 2 seconds on levels 1, 2 and 3; frightened ghosts turn back.
- **Ghosts** move one cell a tick but rest one tick in every 5, 8 and 12 on levels 1, 2 and 3, so Pac-Man is a little faster. They move at half speed while frightened, in the tunnel, or leaving the house, and eyes at full speed.
- **Where a ghost goes.** At each cell a ghost takes the open way, never back the way it came, whose next cell is nearest its target in a straight line, ties broken up, left, down, right. A frightened ghost picks among the same ways at random. Targets:
  - scattering: its own corner off the maze, Blinky top right, Pinky top left, Inky bottom right, Clyde bottom left;
  - chasing: Blinky, Pac-Man's cell; Pinky, 4 cells ahead of Pac-Man; Inky, the cell 2 ahead of Pac-Man, doubled away from Blinky; Clyde, Pac-Man's cell while more than 8 cells away, else its corner;
  - eyes: the cell above the door, then down into the house, where it revives and leaves again.
- **Scatter and chase** alternate on a schedule that restarts each round: level 1 scatters 7 s, chases 20, scatters 7, chases 20, scatters 5, chases 20, scatters 5, then chases for good; level 2 is the same but chases for good after the third scatter; level 3 scatters 5 s instead of 7. A fright holds the schedule still. At each change of mode the ghosts in the maze turn back.
- **The house.** Blinky starts outside it; Pinky leaves at once, Inky after 30 dots or 4 seconds of the round, Clyde after 60 dots or 8 seconds.
- **Collisions** are checked after Pac-Man moves and again after the ghosts move. A frightened ghost Pac-Man meets is eaten: it scores 200, 400, 800, then 1600 for each next ghost of the same fright, and becomes eyes. Eyes pass by. Any other ghost catches Pac-Man.
- **Caught.** Pac-Man loses a life. With lives left, Pac-Man and the ghosts go back to their starts, the dots stay eaten, the round's schedule restarts, and the game pauses until the next turn. With none left the game ends, reason `game_over`: a loss.
- **Level cleared.** Eating the last dot or pellet clears the level, once Pac-Man has met the ghosts on that cell: a frightened ghost there is eaten first, and a ghost that catches Pac-Man there takes a life, the dot staying eaten, so the level clears on the first tick of the next round. Levels 1 and 2 go on to the next level, faster, with a full maze, paused until the next turn. Clearing level 3 ends the game, reason `cleared`: a win.
- **Pause.** `p` pauses a running game; any turn resumes it. The server also pauses the game when your last session closes and when it restarts; no ticks come while paused.

## Randomness
- Only frightened ghosts draw from the rng: the core's seeded source for the tick's step, as ludo and poker draw for their actions. Ticks are steps, so a restart replays every tick and every turn and gets the same game; the restore test checks this.

## Rulings
| Question | Ruling | Why |
|---|---|---|
| Who keeps time | the server ticks the game; the rules read no clock | user ruling: server tick; a restore replays ticks exactly |
| Players | solo only | user ruling |
| Variant | classic-lite: the arcade maze, dots, 4 pellets, the four ghosts' personalities, scatter, chase and fright, 3 lives, levels speed up; no fruit, no cut-scenes, no extra life | user ruling |
| Win and loss | clearing level 3 wins; losing every life first loses | user ruling |
| Score | 10 a dot, 50 a pellet, 200-1600 a ghost; in the view, while the archive outcome stays the solo +1/−1 | plan; the leaderboard means the same for every solo game |
| Grid | whole cells, one per tick; ghosts decide at their own cell, not one ahead | a cell grid keeps the wire and the replay simple |
| Speeds | Pac-Man every tick; ghosts rest one tick in 5/8/12; half speed when frightened, in the tunnel or leaving the house | the arcade's ratios, rounded to ticks |
| Arcade quirks | no "no turning up" zones above the house, no Pinky overflow bug, no flashing before a fright ends | classic-lite |
| A caught life or a cleared level | pauses until the next turn | the player gets back in step with a board that has just reset |
| Start | paused until the first turn | the board opens before the game moves |
| Input | `turn` with `up`, `down`, `left` or `right`, held until it can be taken; `pause` | the catalogue's values are what the parser accepts |

## Other endings (core)
- Give up (`R`, confirms): a loss with no winner, reason `resign`.
- Abort (`A`): only before your first turn; result `aborted`, scores nothing.
- After either, `can_turn` and `can_pause` are false and no tick comes, because pacman implements `game.Concluder`.

## Options
None.

## Controls and keys
Buffer-local, normal mode, in the board buffer.

| Key | Action |
|---|---|
| `h` | turn left |
| `j` | turn down |
| `k` | turn up |
| `l` | turn right |
| `p` | pause, while running |
| `R` | give up |
| `A` | abort, before your first turn |
| `]g` | next game waiting on your move |
| `q` | close (the game goes on; press `p` first) |

## Display
- Each cell is two columns wide, and cells touch, so the walls join. Walls are `██` (ascii `##`), the door `──`, a dot `·`, a pellet `●` (ascii `.` and `o`).
- Pac-Man's mouth faces its way: `ᗧ` `ᗤ` `ᗢ` `ᗣ` for right, left, up and down (ascii `<` `>` `V` `^`), in `GamesPacman`.
- Each ghost shows its letter, `B`, `P`, `I` or `C`, after `ᗝ` (ascii `M`), in its own colour. A frightened ghost's letter is lower case and in `GamesGhostFrightened`; eyes are a quote before the letter, in `GamesMuted`. So neither state rests on colour alone.
- A panel beside the board shows `score`, `lives`, `level` (as `2/3`), `food` left, and `game`: `running`, `paused: turn to go`, or `over`.
- In a window tall enough for two-line cells (about 70 lines) the walls are drawn solid in block art; everything else keeps its glyph.
- Events notify as `alice ate pinky +400`, `blinky caught alice, 2 lives left` and `alice cleared level 1`, when the board is not shown.

## Wire
- **Actions:** `{"kind": "turn", "direction": "left"}`, `{"kind": "pause"}`.
- **View:** `{maze, pacman, ghosts, score, lives, level, levels, food_left, fright_ticks, paused, can_turn, can_pause}`. See `docs/protocol.md` and `docs/schema/games/pacman.json`.
- **Updates:** one per tick, several a second, each with the whole view (about 2 KB).
- **Errors:**

| Case | Message |
|---|---|
| `pause` while paused | `already paused` |
| not up, down, left or right | `want up, down, left or right; got "<text>"` |
| unknown action | `unknown action: <kind>` |
| the game is over | `not your turn` |

## Scoring and result
Win +1 (level 3 cleared), loss −1 (every life lost, or giving up), abort 0. The points stay in the view. Solo games count in the pacman records but not in the all-games leaderboard.

## Known limits
- The board is 31 lines tall plus 5 of chrome, so it needs a window of about 36 lines; a shorter one cuts the bottom.
- Closing the board with `q` does not pause the game while the session stays connected; press `p` first.
- Every tick is a stored step, about 8 a second, so an hour's play is some 25 000 rows until the game finishes and they are replaced by its summary. They are not coalesced.
- No fruit, no extra life, no flashing before a fright ends, and only three levels.
- The conformance transcript ticks a few times and gives up; playing to a finish is covered by the Go conformance and the smoke test.
