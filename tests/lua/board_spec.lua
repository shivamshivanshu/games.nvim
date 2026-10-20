local chess = require("games.games.chess")
local helpers = require("helpers")
local shell = require("games.ui.shell")
local cases = require("fixtures.chess_h1")

local assert_eq = helpers.assert_eq

local HEADER_LINES = 3
local BODY_CHROME_LINES = 3
local LABEL_LINES = 1

---@param history string[]
---@return games.GameUpdate
local function with_history(history)
  local update = vim.deepcopy(cases.start_unicode.update)
  update.view.history_san = history
  return update
end

---Cell height of a chess board rendered into a text area, from its line count.
---@param update games.GameUpdate
---@param text_area games.Size
---@param max_cell_height? integer
---@return integer
---@return string[] lines
local function cell_height(update, text_area, max_cell_height)
  local view =
    vim.tbl_extend("force", helpers.chess_view("unicode"), { max_cell_height = max_cell_height })
  local display = { text_area = text_area, ambiwidth = vim.o.ambiwidth }
  local rendered = shell.render(update, chess, view, "alice", 0, display)
  local board_lines = #rendered.lines - HEADER_LINES - BODY_CHROME_LINES - LABEL_LINES
  return board_lines / 8, rendered.lines
end

return {
  board_fills_its_window_by_cell_height = function()
    local start = with_history({})
    for _, case in ipairs({
      { text_area = { w = 60, h = 15 }, expected = 1 },
      { text_area = { w = 80, h = 24 }, expected = 2 },
      { text_area = { w = 120, h = 40 }, expected = 4 },
      { text_area = { w = 200, h = 60 }, expected = 6 },
      { text_area = { w = 80, h = 60 }, expected = 2 },
      { text_area = { w = 200, h = 60 }, max_cell_height = 3, expected = 3 },
      { text_area = { w = 200, h = 60 }, max_cell_height = 1, expected = 1 },
    }) do
      local height = cell_height(start, case.text_area, case.max_cell_height)
      assert_eq({ case, case.expected }, { case, height })
    end
  end,

  a_growing_move_list_never_resizes_the_board = function()
    local area = { w = 100, h = 40 }
    local moves = {}
    for i = 1, 160 do
      moves[i] = i % 2 == 1 and "Nbxd7+" or "exd8=Q#"
    end
    local empty_height = cell_height(with_history({}), area)
    local long_height, lines = cell_height(with_history(moves), area)

    assert_eq(empty_height, long_height)
    local text = table.concat(lines, "\n")
    assert(text:find("80. Nbxd7+ exd8=Q#", 1, true), text)
    assert(not text:find(" 1. Nbxd7+", 1, true), text)
  end,

  large_board_draws_block_art_except_under_ambiwidth_double = function()
    local block = "\226\150[\128-\159]"
    local _, single = cell_height(with_history({}), { w = 200, h = 60 })
    assert(table.concat(single):find(block), "no block art at cell height 6")
    helpers.with_overrides({ { vim.o, "ambiwidth", "double" } }, function()
      local _, double = cell_height(with_history({}), { w = 200, h = 60 })
      local text = table.concat(double, "\n")
      assert(not text:find(block) and not text:find("·", 1, true), text)
      assert(text:find("♜", 1, true) and text:find("%s%.%s"), text)
    end)
  end,

  a_banner_never_resizes_the_board = function()
    local area = { w = 200, h = 46 }
    local offered = with_history({})
    offered.draw_agreed = { "bob" }
    local _, plain = cell_height(with_history({}), area)
    local _, banner = cell_height(offered, area)
    assert_eq(#plain, #banner)
  end,
}
