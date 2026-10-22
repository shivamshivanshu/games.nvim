local board = require("games.ui.widgets.board")
local chess = require("games.games.chess")
local compose = require("games.ui.compose")
local helpers = require("helpers")
local key_hints = require("games.ui.widgets.key_hints")
local shell = require("games.ui.shell")
local cases = require("fixtures.chess_h1")

local assert_eq = helpers.assert_eq

local HEADER_LINES = 3
local BODY_BLANK_LINES = 2
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
  local view = helpers.view_options("chess", "unicode", { max_cell_height = max_cell_height })
  local display = { text_area = text_area, ambiwidth = vim.o.ambiwidth }
  local rendered, _, bindings = shell.render(update, chess, view, "alice", 0, display)
  local footer_lines = compose.measure(key_hints(bindings, text_area.w)).h
  local chrome = HEADER_LINES + BODY_BLANK_LINES + footer_lines + LABEL_LINES
  local board_lines = #rendered.lines - chrome
  return board_lines / 8, rendered.lines
end

local T = {}

function T.each_grid_of_a_board_keeps_its_own_rows()
  ---@param glyph string
  ---@return games.GridProps
  local function same_keyed(glyph)
    return {
      rows = 2,
      cols = 3,
      cell = function()
        return { glyph }
      end,
      row_key = function()
        return "same"
      end,
    }
  end
  local props = { grid = same_keyed("o"), beside = { same_keyed("x") } }
  local function draw(grid_rows)
    local ctx = { me = "alice", ambiwidth = "single", grid_rows = grid_rows }
    return compose.render(board(props, ctx --[[@as games.LayoutCtx]])).lines
  end
  local afresh, grid_rows = draw(nil), {}
  assert_eq(afresh, draw(grid_rows))
  assert_eq(afresh, draw(grid_rows))
end

function T.board_fills_its_window_by_cell_height()
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
end

function T.a_growing_move_list_never_resizes_the_board()
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
end

function T.large_board_draws_block_art_except_under_ambiwidth_double()
  local block = "\226\150[\128-\159]"
  local _, single = cell_height(with_history({}), { w = 200, h = 60 })
  assert(table.concat(single):find(block), "no block art at cell height 6")
  helpers.with_overrides({ { vim.o, "ambiwidth", "double" } }, function()
    local _, double = cell_height(with_history({}), { w = 200, h = 60 })
    local text = table.concat(double, "\n")
    assert(not text:find(block) and not text:find("·", 1, true), text)
    assert(text:find("♜", 1, true) and text:find("%s%.%s"), text)
  end)
end

function T.an_unsized_panel_is_drawn_at_the_board_height()
  local panel = {
    node = function(max_height)
      local lines = {}
      for i = 1, max_height do
        lines[i] = compose.text("p")
      end
      return compose.col(lines)
    end,
  }
  local grid = {
    rows = 2,
    cols = 2,
    shade = "plain",
    labels = { cols = "ab", rows = "12" },
    cell = function()
      return { "x" }
    end,
  }
  local ctx = { space = { w = 40, h = 12 }, ambiwidth = "single" }
  local alone = compose.measure(board({ grid = grid }, ctx))
  local beside = compose.measure(board({ grid = grid, panels = { panel } }, ctx))

  assert(alone.h < ctx.space.h, vim.inspect(alone))
  assert_eq(alone.h, beside.h)
end

function T.a_board_draws_from_the_ctx_art_and_shades_plain_by_default()
  local grid = {
    rows = 1,
    cols = 2,
    cell = function(_, col)
      return { col == 1 and "x" or "o" }
    end,
  }
  local art =
    { x = { { h = 1, w = 1, lines = { "X" } } }, o = { { h = 1, w = 1, lines = { "O" } } } }
  local rendered = compose.render(board({ grid = grid }, { art = art, ambiwidth = "single" }))

  assert_eq({ "X O" }, rendered.lines)
  assert_eq({}, helpers.extmarks(rendered))
end

function T.a_banner_never_resizes_the_board()
  local area = { w = 200, h = 46 }
  local offered = with_history({})
  offered.draw_agreed = { "bob" }
  local _, plain = cell_height(with_history({}), area)
  local _, banner = cell_height(offered, area)
  assert_eq(#plain, #banner)
end

function T.grids_beside_the_first_share_its_cell_height_under_their_titles()
  local function small(key)
    return {
      rows = 2,
      cols = 2,
      shade = "plain",
      labels = { cols = "ab", rows = "12" },
      cell = function()
        return { key }
      end,
    }
  end
  local props = { grid = small("x"), beside = { small("o") }, titles = { "mine", "theirs" } }
  local panel = {
    width = 4,
    node = function()
      return compose.text("note")
    end,
  }
  local narrow = compose.render(board(vim.tbl_extend("force", props, { panels = { panel } }), {}))
  assert_eq({
    "mine    theirs   note",
    "1 x x   1 o o",
    "2 x x   2 o o",
    "  a b     a b",
  }, helpers.trimmed_lines(narrow))

  local roomy = compose.render(board(props, { space = { w = 40, h = 9 }, ambiwidth = "single" }))
  local cell_height = 3
  assert_eq(1 + 2 * cell_height + 1, #roomy.lines)
  for i, line in ipairs(roomy.lines) do
    assert_eq({ i, line:find("x", 1, true) ~= nil }, { i, line:find("o", 1, true) ~= nil })
  end
end

return T
