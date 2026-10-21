# TODO

- Room passphrase as invite (parked 2026-10-20): host sets a passphrase; players self-register with it (`:Games register`), login stays user+password, `adduser` kept as admin fallback. Notes: /home/claude_notes/scratchpad/20261020/games-nvim-passphrase-plan.md
- `game.start` has no per-user cap on ongoing games: one user can start solo games without limit. Found in core-solo review.
- WebSocket gateway (later), for a browser client: bridge WS frames to the NDJSON protocol in docs/protocol.md. It must pass the client IP through, via the PROXY protocol or a trusted forwarded header, because login lockout is per IP; without it every browser shares the gateway's IP and one bad password locks out everyone.
- Client finished-game cache evicts the newest on a `game.list finished`: the server lists newest first, `store_listed_games` stores in list order, and `keep_finished` drops the oldest *stored*, so of 50 listed it keeps the 32 oldest. Evict by `finished_at` instead. The dashboard and finder read the listed games directly, so they are unaffected. Found in C13.
- Lift Wordle's block-letter `FONT` and sprite code out of `lua/games/assets/wordle/default.lua` into `lua/games/assets/shared/`; hangman's pack requires `games.assets.wordle.default` for its letters today. For the parked simplify. Found in L1 (hangman).
- Images via kitty graphics (parked by user 2026-10-21): opt-in `glyphs = "image"` via snacks image placeholders; chess PoC first. Recon: /home/claude_notes/scratchpad/20261021/games-nvim-kitty-images-recon.md
- Deferred games (user, 2026-10-21): Quoridor, Backgammon (rulings in spec §3.9; Backgammon reuses game/dice, Quoridor the grid edge layer from Dots and Boxes), plus Go, Shogi/Xiangqi, Hex.
