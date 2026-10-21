local chess = require("games.games.chess")
local helpers = require("helpers")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local NONE = { pieces = {}, score = 0 }
local PROMOTED_FEN = "rnbqkbnr/pppppppp/8/8/4Q3/8/PPPPPPP1/RNBQKBNR w KQkq - 0 9"

local T = {}

function T.material_shows_only_the_surplus_of_the_side_ahead_per_kind()
  for _, case in ipairs({
    {
      name = "start position",
      fen = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
      white = NONE,
      black = NONE,
    },
    {
      name = "white took a knight",
      fen = "r1bqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 3",
      white = { pieces = { "n" }, score = 3 },
      black = NONE,
    },
    {
      name = "knights traded",
      fen = "r1bqkbnr/pppppppp/8/8/8/8/PPPPPPPP/R1BQKBNR w KQkq - 0 5",
      white = NONE,
      black = NONE,
    },
    {
      name = "white promoted a pawn",
      fen = PROMOTED_FEN,
      white = { pieces = { "q" }, score = 8 },
      black = { pieces = { "P" }, score = 0 },
    },
    {
      name = "white up a rook, black up a bishop and a knight",
      fen = "1nbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/R2QKBNR w Kk - 0 9",
      white = { pieces = { "r" }, score = 0 },
      black = { pieces = { "B", "N" }, score = 1 },
    },
  }) do
    assert_eq(
      { case.name, { white = case.white, black = case.black } },
      { case.name, chess.material(case.fen) }
    )
  end
end

function T.header_shows_each_sides_surplus_in_the_opponents_glyphs_after_the_clock()
  local update = helpers.chess_update({
    clock = { remaining_ms = { alice = 300000, bob = 300000 }, running = "alice" },
    view = { fen = PROMOTED_FEN, history_san = {}, legal_moves = {}, claimable_draw = false },
  })
  for _, case in ipairs({
    {
      glyphs = "unicode",
      header = {
        "alice  white  5:00  ♛ +8  ←  your move ",
        "bob    black  5:00  ♙                  ",
      },
    },
    {
      glyphs = "ascii",
      header = {
        "alice  white  5:00  q +8  ←  your move ",
        "bob    black  5:00  P                  ",
      },
    },
  }) do
    local rendered, header_height =
      shell.render(update, chess, helpers.view_options("chess", chess, case.glyphs), "alice")
    local marks = vim.tbl_filter(function(mark)
      return mark.hl == "GamesMuted" and mark.col == #"alice  white  5:00  "
    end, rendered.extmarks)

    assert_eq(case.header, vim.list_slice(rendered.lines, 1, 2))
    assert_eq(3, header_height)
    assert_eq(
      { 0, 1 },
      vim.tbl_map(function(mark)
        return mark.row
      end, marks)
    )
  end
end

return T
