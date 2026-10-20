# TODO

- Room passphrase as invite (parked 2026-10-20): host sets a passphrase; players self-register with it (`:Games register`), login stays user+password, `adduser` kept as admin fallback. Notes: /home/claude_notes/scratchpad/20261020/games-nvim-passphrase-plan.md
- `game.start` has no per-user cap on ongoing games: one user can start solo games without limit. Found in core-solo review.
- WebSocket gateway (later), for a browser client: bridge WS frames to the NDJSON protocol in docs/protocol.md. It must pass the client IP through, via the PROXY protocol or a trusted forwarded header, because login lockout is per IP; without it every browser shares the gateway's IP and one bad password locks out everyone.
