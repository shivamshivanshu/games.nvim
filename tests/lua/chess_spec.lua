local chess = require("games.games.chess")
local helpers = require("helpers")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local PROMOTED_FEN = "rnbqkbnr/pppppppp/8/8/4Q3/8/PPPPPPP1/RNBQKBNR w KQkq - 0 9"

local T = {}

function T.header_shows_each_sides_surplus_in_the_opponents_glyphs_after_the_clock()
  local update = helpers.chess_update({
    clock = { remaining_ms = { alice = 300000, bob = 300000 }, running = "alice" },
    view = {
      fen = PROMOTED_FEN,
      material = {
        white = { pieces = { "q" }, score = 8 },
        black = { pieces = { "P" }, score = 0 },
      },
    },
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
    end, helpers.extmarks(rendered))

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
