---Chess renders pinned byte for byte at cell height 1; chess_h1.json holds their output from mvp
---before scalable boards.

local START_FEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
local FOOLS_MATE_FEN = "rnb1kbnr/pppp1ppp/8/4p3/6Pq/5P2/PPPPP2P/RNBQKBNR w KQkq - 1 3"
local ITALIAN_FEN = "r1bqk1nr/pppp1ppp/2n5/2b1p3/2B1P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 4 4"

local function update(overrides)
  return vim.tbl_extend("force", {
    game_id = "0000",
    game = "chess",
    seq = 0,
    options = {},
    players = { { user = "alice", seat = "white" }, { user = "bob", seat = "black" } },
    to_act = { "alice" },
    controls = { "resign", "offer_draw", "abort" },
    events = {},
    view = { fen = START_FEN, history_san = {}, in_check = false },
  }, overrides)
end

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
  finished_without_view = {
    update = finished,
    glyphs = "unicode",
    flipped = false,
    me = "alice",
    elapsed_ms = 0,
  },
}
