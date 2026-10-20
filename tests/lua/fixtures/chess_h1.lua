---Chess renders pinned byte for byte at cell height 1. chess_h1.json holds mvp's output from
---before scalable boards, changed only by rulings R1 (a blank status line without a banner) and
---R2 (the move list shows its tail within the board's height).

local update = require("helpers").chess_update

local FOOLS_MATE_FEN = "rnb1kbnr/pppp1ppp/8/4p3/6Pq/5P2/PPPPP2P/RNBQKBNR w KQkq - 1 3"
local ITALIAN_FEN = "r1bqk1nr/pppp1ppp/2n5/2b1p3/2B1P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 4 4"

local fools_mate = update({
  seq = 4,
  to_act = {},
  controls = {},
  result = { kind = "decisive", winners = { "bob" }, reason = "checkmate" },
  view = {
    fen = FOOLS_MATE_FEN,
    history_san = { "f3", "e5", "g4", "Qh4#" },
    last_move = { from = "d8", to = "h4" },
    in_check = true,
  },
})

local italian = update({
  seq = 6,
  clock = { remaining_ms = { alice = 9000, bob = 300000 }, running = "alice" },
  draw_agreed = { "bob" },
  controls = { "resign", "offer_draw", "decline_draw" },
  view = {
    fen = ITALIAN_FEN,
    history_san = { "e4", "e5", "Nf3", "Nc6", "Bc4", "Bc5" },
    last_move = { from = "f8", to = "c5" },
    in_check = false,
  },
})

local long_game = update({
  seq = 40,
  view = {
    fen = "r4rk1/1b1n1pp1/3n1q1p/2p5/1p6/1B3N1P/PP1N1PP1/R2QR1K1 w - - 0 21",
    history_san = vim.split(
      "e4 e5 Nf3 Nc6 Bb5 a6 Ba4 Nf6 O-O Be7 Re1 b5 Bb3 d6 c3 O-O h3 Nb8 d4 Nbd7 c4 c6 cxb5 axb5 "
        .. "Nc3 Bb7 Bg5 b4 Nb1 h6 Bh4 c5 dxe5 Nxe4 Bxe7 Qxe7 exd6 Qf6 Nbd2 Nxd6",
      " "
    ),
    last_move = { from = "e4", to = "d6" },
    in_check = false,
  },
})

local finished = update({ seq = 4, to_act = {}, controls = {}, result = fools_mate.result })
finished.view = nil

---@class games.test.GoldenCase
---@field update games.GameUpdate
---@field glyphs games.Glyphs
---@field flipped boolean
---@field me games.UserID
---@field elapsed_ms integer

---@type table<string, games.test.GoldenCase>
return {
  start_unicode = {
    update = update({}),
    glyphs = "unicode",
    flipped = false,
    me = "alice",
    elapsed_ms = 0,
  },
  start_ascii_black = {
    update = update({}),
    glyphs = "ascii",
    flipped = false,
    me = "bob",
    elapsed_ms = 0,
  },
  start_unicode_flipped = {
    update = update({}),
    glyphs = "unicode",
    flipped = true,
    me = "alice",
    elapsed_ms = 0,
  },
  fools_mate_unicode = {
    update = fools_mate,
    glyphs = "unicode",
    flipped = false,
    me = "alice",
    elapsed_ms = 0,
  },
  fools_mate_ascii_black = {
    update = fools_mate,
    glyphs = "ascii",
    flipped = false,
    me = "bob",
    elapsed_ms = 0,
  },
  italian_clock_unicode = {
    update = italian,
    glyphs = "unicode",
    flipped = false,
    me = "alice",
    elapsed_ms = 1500,
  },
  italian_clock_ascii = {
    update = italian,
    glyphs = "ascii",
    flipped = false,
    me = "bob",
    elapsed_ms = 0,
  },
  long_game_unicode = {
    update = long_game,
    glyphs = "unicode",
    flipped = false,
    me = "alice",
    elapsed_ms = 0,
  },
  finished_without_view = {
    update = finished,
    glyphs = "unicode",
    flipped = false,
    me = "alice",
    elapsed_ms = 0,
  },
}
