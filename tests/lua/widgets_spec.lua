local compose = require("games.ui.compose")
local grid = require("games.ui.widgets.grid")
local movelist = require("games.ui.widgets.movelist")

local function small_grid(overrides)
  return grid(vim.tbl_extend("force", {
    rows = 2,
    cols = 2,
    shade = "checker",
    labels = { cols = "ab", rows = "21" },
    cell = function()
      return { "x" }
    end,
  }, overrides or {}))
end

local function hl_at(rendered, row, col)
  for _, mark in ipairs(rendered.extmarks) do
    if mark.row == row and mark.col == col then
      return mark.hl
    end
  end
end

local function assert_eq(expected, actual)
  assert(vim.deep_equal(expected, actual), vim.inspect({ expected = expected, actual = actual }))
end

local function assert_uniform_width(lines)
  local width = vim.fn.strdisplaywidth(lines[1])
  for _, line in ipairs(lines) do
    assert_eq(width, vim.fn.strdisplaywidth(line))
  end
end

local function board(glyph)
  return grid({
    rows = 8,
    cols = 8,
    shade = "checker",
    labels = { cols = "abcdefgh", rows = "87654321" },
    cell = function()
      return { glyph }
    end,
  })
end

local function assert_movelist_aligned(glyph)
  local moves = { "e4", "e5", "Nf3", "Nc6" }
  local rendered = compose.render(
    compose.row({ board(glyph), compose.pad(3, movelist({ items = moves, columns = 2 })) })
  )
  assert_uniform_width(rendered.lines)
  local board_width = vim.fn.strdisplaywidth(compose.render(board(glyph)).lines[1])
  local prefix = rendered.lines[1]:match("^(.-)e4")
  assert_eq(board_width + 3, vim.fn.strdisplaywidth(prefix))
  prefix = rendered.lines[2]:match("^(.-)Nf3")
  assert_eq(board_width + 3, vim.fn.strdisplaywidth(prefix))
end

return {
  grid_checker_and_labels = function()
    local rendered = compose.render(small_grid())
    assert_eq({ "2 x x", "1 x x", "  a b" }, rendered.lines)
    assert_eq("GamesSquareLight", hl_at(rendered, 0, 2))
    assert_eq("GamesSquareDark", hl_at(rendered, 0, 4))
  end,

  grid_flip = function()
    local rendered = compose.render(small_grid({
      flip = true,
      cell = function(r, c)
        return { ({ { "p", "q" }, { "r", "s" } })[r][c] }
      end,
    }))
    assert_eq({ "1 s r", "2 q p", "  b a" }, rendered.lines)
  end,

  grid_mark_overrides_shade = function()
    local rendered = compose.render(small_grid({ marks = { ["1,1"] = "GamesLastMove" } }))
    assert_eq("GamesLastMove", hl_at(rendered, 0, 2))
    assert_eq("GamesSquareDark", hl_at(rendered, 0, 4))
  end,

  row_pads_mixed_width = function()
    assert_movelist_aligned("x")
    assert_movelist_aligned("♟")
  end,

  extmark_columns_are_bytes_after_multibyte_glyph = function()
    local rendered = compose.render(small_grid({
      cell = function()
        return { "♟" }
      end,
    }))
    assert_eq({ "2 ♟ ♟", "1 ♟ ♟", "  a b" }, rendered.lines)
    local second = rendered.extmarks[2]
    assert_eq({ row = 0, col = 6, end_col = 9, hl = "GamesSquareDark" }, second)
  end,

  grid_cell_width_two_with_wide_glyph = function()
    local rendered = compose.render(small_grid({
      cell_width = 2,
      cell = function()
        return { "中" }
      end,
    }))
    assert_eq({ "2 中 中", "1 中 中", "  a  b " }, rendered.lines)
    assert_uniform_width(rendered.lines)
  end,

  grid_labels_split_by_character = function()
    local rendered = compose.render(small_grid({ labels = { cols = "①②", rows = "③④" } }))
    assert_eq({ "③ x x", "④ x x", "  ① ②" }, rendered.lines)
  end,
}
