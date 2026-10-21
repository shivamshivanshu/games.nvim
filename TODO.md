# TODO

- Room passphrase as invite (parked 2026-10-20): host sets a passphrase; players self-register with it (`:Games register`), login stays user+password, `adduser` kept as admin fallback. Notes: /home/claude_notes/scratchpad/20261020/games-nvim-passphrase-plan.md
- `game.start` has no per-user cap on ongoing games: one user can start solo games without limit. Found in core-solo review.
- WebSocket gateway (later), for a browser client: bridge WS frames to the NDJSON protocol in docs/protocol.md. It must pass the client IP through, via the PROXY protocol or a trusted forwarded header, because login lockout is per IP; without it every browser shares the gateway's IP and one bad password locks out everyone.
- Images via kitty graphics (parked by user 2026-10-21): opt-in `glyphs = "image"` via snacks image placeholders; chess PoC first. Recon: /home/claude_notes/scratchpad/20261021/games-nvim-kitty-images-recon.md
- Deferred games (user, 2026-10-21): Quoridor, Backgammon (rulings in spec §3.9; Backgammon reuses game/dice, Quoridor the grid edge layer from Dots and Boxes), plus Go, Shogi/Xiangqi, Hex.
- In a simultaneous phase (battleship setup) a `<CR>` racing the opponent's push gets `stale`, and `snapshot_if_stale` (`lua/games/core/client.lua:101`) drops it silently; resend or tell the player. Found in M1 review.
