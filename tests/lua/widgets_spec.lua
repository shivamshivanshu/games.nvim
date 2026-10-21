local compose = require("games.ui.compose")
local grid = require("games.ui.widgets.grid")
local movelist = require("games.ui.widgets.movelist")
local helpers = require("helpers")
local key_hints = require("games.ui.widgets.key_hints")
local kv = require("games.ui.widgets.kv")

local assert_eq = helpers.assert_eq

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

---A 6x6 grid in 3x3 blocks whose cells show (row + col) mod 10.
---@param overrides table
---@return games.Node
local function digit_grid(overrides)
  return grid(vim.tbl_extend("force", {
    rows = 6,
    cols = 6,
    shade = "plain",
    blocks = 3,
    labels = { cols = "abcdef", rows = "654321" },
    cell = function(r, c)
      return { tostring((r + c) % 10) }
    end,
  }, overrides))
end

local TALL_ART = {
  A = { { h = 1, w = 1, lines = { "a" } }, { h = 3, w = 4, lines = { "/--\\", "|  |", "\\--/" } } },
  B = { { h = 1, w = 1, lines = { "b" } } },
}

---A 2x2 grid of 3x6 cells, A on the diagonal and B off it.
---@param overrides table
---@return games.Node
local function tall_grid(overrides)
  return small_grid(vim.tbl_extend("force", {
    cell_width = 6,
    cell_height = 3,
    art = TALL_ART,
    cell = function(r, c)
      return { r == c and "A" or "B" }
    end,
  }, overrides))
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

local T = {}

function T.grid_checker_and_labels()
  local rendered = compose.render(small_grid())
  assert_eq({ "2 x x", "1 x x", "  a b" }, rendered.lines)
  assert_eq({ "GamesSquareLight", "GamesSquareLightInk" }, helpers.hl_at(rendered, 0, 2))
  assert_eq({ "GamesSquareDark", "GamesSquareDarkInk" }, helpers.hl_at(rendered, 0, 4))
end

function T.grid_redraws_a_kept_row_only_when_its_key_changes()
  local glyphs, keys = { { "p", "q" }, { "r", "s" } }, { "1", "2" }
  local kept = {}
  local function draw()
    return compose.render(small_grid({
      kept = kept,
      cell = function(r, c)
        return { glyphs[r][c] }
      end,
      row_key = function(r)
        return keys[r]
      end,
    })).lines
  end
  assert_eq({ "2 p q", "1 r s", "  a b" }, draw())
  glyphs[1][1], glyphs[2][2] = "x", "y"
  keys[2] = "2'"
  assert_eq({ "2 p q", "1 r y", "  a b" }, draw())
end

function T.flipped_grid_keeps_no_row_across_a_change_of_its_row_count()
  local kept = {}
  local function draw(rows, labels, kept_rows)
    return helpers.extmarks(compose.render(small_grid({
      rows = rows,
      flip = true,
      labels = { cols = "ab", rows = labels },
      kept = kept_rows,
      row_key = function()
        return "same"
      end,
    })))
  end
  draw(2, "21", kept)
  assert_eq(draw(3, "321", nil), draw(3, "321", kept))
end

function T.grid_gap_sets_the_columns_between_one_line_cells()
  local touching = compose.render(small_grid({
    gap = 0,
    cell_width = 2,
    cell = function()
      return { "##" }
    end,
  }))
  assert_eq({ "2 ####", "1 ####", "  a b " }, touching.lines)
  assert_eq({ "2 x   x", "1 x   x", "  a   b" }, compose.render(small_grid({ gap = 3 })).lines)
end

function T.grid_flip()
  local rendered = compose.render(small_grid({
    flip = true,
    cell = function(r, c)
      return { ({ { "p", "q" }, { "r", "s" } })[r][c] }
    end,
  }))
  assert_eq({ "1 s r", "2 q p", "  b a" }, rendered.lines)
end

function T.grid_layers_shade_then_mark_then_cell_foreground()
  local rendered = compose.render(small_grid({
    cell = function()
      return { "x", "GamesPieceWhite" }
    end,
    marks = { { row = 1, col = 1, hl = { "GamesLastMove", "GamesLastMoveDark" } } },
  }))
  assert_eq(
    { "GamesSquareLight", "GamesLastMove", "GamesLastMoveDark", "GamesPieceWhite" },
    helpers.hl_at(rendered, 0, 2)
  )
  assert_eq({ "GamesSquareDark", "GamesPieceWhite" }, helpers.hl_at(rendered, 0, 4))
end

function T.row_pads_mixed_width()
  assert_movelist_aligned("x")
  assert_movelist_aligned("♟")
end

function T.extmark_columns_are_bytes_after_multibyte_glyph()
  local rendered = compose.render(small_grid({
    cell = function()
      return { "♟" }
    end,
  }))
  assert_eq({ "2 ♟ ♟", "1 ♟ ♟", "  a b" }, rendered.lines)
  local second = helpers.extmarks(rendered)[2]
  assert_eq(
    { row = 0, col = 6, end_col = 9, hl = { "GamesSquareDark", "GamesSquareDarkInk" } },
    second
  )
end

function T.grid_cell_width_two_with_wide_glyph()
  local rendered = compose.render(small_grid({
    cell_width = 2,
    cell = function()
      return { "中" }
    end,
  }))
  assert_eq({ "2 中 中", "1 中 中", "  a  b " }, rendered.lines)
  assert_uniform_width(rendered.lines)
end

function T.grid_blocks_draw_separators_between_boxes()
  local rendered = compose.render(digit_grid({}))
  assert_eq({
    "6 2 3 4│5 6 7",
    "5 3 4 5│6 7 8",
    "4 4 5 6│7 8 9",
    "  ─────┼─────",
    "3 5 6 7│8 9 0",
    "2 6 7 8│9 0 1",
    "1 7 8 9│0 1 2",
    "  a b c d e f",
  }, rendered.lines)
  assert_uniform_width(rendered.lines)
  assert_eq("GamesMuted", helpers.hl_at(rendered, 0, 7))
end

function T.grid_blocks_use_ascii_separators_in_ascii_mode()
  local rendered = compose.render(digit_grid({ glyphs = "ascii" }))
  assert_eq({
    "6 2 3 4|5 6 7",
    "5 3 4 5|6 7 8",
    "4 4 5 6|7 8 9",
    "  -----+-----",
    "3 5 6 7|8 9 0",
    "2 6 7 8|9 0 1",
    "1 7 8 9|0 1 2",
    "  a b c d e f",
  }, rendered.lines)
end

function T.grid_blocks_fall_back_to_ascii_separators_under_ambiwidth_double()
  helpers.with_overrides({ { vim.o, "ambiwidth", "double" } }, function()
    local rendered = compose.render(digit_grid({ ambiwidth = "double" }))
    assert_eq("6 2 3 4|5 6 7", rendered.lines[1])
    assert_eq("  -----+-----", rendered.lines[4])
    assert_uniform_width(rendered.lines)
  end)
end

function T.grid_rejects_blocks_that_are_not_positive_integers()
  for _, blocks in ipairs({ 0, -3, 1.5 }) do
    local ok, err = pcall(digit_grid, { blocks = blocks })
    assert(not ok and err:find("blocks must be a positive integer", 1, true), vim.inspect(err))
  end
end

function T.grid_tall_cells_centre_art_and_label_the_middle_line()
  local rendered = compose.render(tall_grid({}))
  assert_eq({
    "   /--\\       ",
    "2  |  |   b   ",
    "   \\--/       ",
    "         /--\\ ",
    "1   b    |  | ",
    "         \\--/ ",
    "    a     b   ",
  }, rendered.lines)
end

function T.grid_flipped_tall_cells_keep_art_upright()
  local rendered = compose.render(tall_grid({
    flip = true,
    cell = function(r, c)
      return { (r == 2 and c == 2) and "A" or "B" }
    end,
  }))
  assert_eq(
    { "   /--\\       ", "1  |  |   b   ", "   \\--/       " },
    vim.list_slice(rendered.lines, 1, 3)
  )
  assert_eq("    b     a   ", rendered.lines[7])
end

function T.grid_mark_tints_every_line_of_a_tall_cell()
  local rendered =
    compose.render(tall_grid({ marks = { { row = 1, col = 1, hl = "GamesLastMove" } } }))
  for row = 0, 2 do
    local expected = { "GamesSquareLight", "GamesLastMove", "GamesLastMoveInk" }
    assert_eq(expected, helpers.hl_at(rendered, row, 2))
  end
  assert_eq({ "GamesSquareDark", "GamesSquareDarkInk" }, helpers.hl_at(rendered, 1, 8))
end

function T.grid_tall_cells_repeat_block_separators_and_reserve_widest_label()
  local rendered = compose.render(small_grid({
    cell_width = 2,
    cell_height = 2,
    blocks = 1,
    labels = { cols = { "a", "b" }, rows = { "10", "9" } },
  }))
  assert_eq({
    "10 x │x ",
    "     │  ",
    "   ──┼──",
    "9  x │x ",
    "     │  ",
    "   a  b ",
  }, rendered.lines)
  assert_eq("GamesMuted", helpers.hl_at(rendered, 1, 5))
end

function T.grid_labels_split_by_character()
  local rendered = compose.render(small_grid({ labels = { cols = "①②", rows = "③④" } }))
  assert_eq({ "③ x x", "④ x x", "  ① ②" }, rendered.lines)
end

function T.grid_edges_draw_a_dot_at_every_corner_and_a_line_along_every_side()
  local rendered = compose.render(small_grid({
    shade = "plain",
    cell_width = 3,
    labels = { cols = "abc", rows = "123" },
    cell = function(r, c)
      return { r == 1 and c == 1 and "A" or " " }
    end,
    edges = {
      across = function(r, c)
        return r == 2 and c == 2 and { " " }
          or { "-", r == 1 and c == 1 and "GamesLastMove" or nil }
      end,
      down = function(r, c)
        return c == 2 and r == 2 and { " " } or { "|" }
      end,
      dot = function()
        return { "+" }
      end,
    },
  }))
  assert_eq({
    "1 +---+---+",
    "  | A |   |",
    "2 +---+   +",
    "  |       |",
    "3 +---+---+",
    "  a   b   c",
  }, rendered.lines)
  assert_eq("GamesLastMove", helpers.hl_at(rendered, 0, 3))
  assert_eq(nil, helpers.hl_at(rendered, 0, 7))
end

function T.grid_edges_resolve_glyphs_and_run_the_height_of_tall_cells()
  local function glyph(line)
    return { { h = 1, w = 1, lines = { line } } }
  end
  local rendered = compose.render(small_grid({
    shade = "plain",
    cell_width = 4,
    cell_height = 2,
    labels = false,
    art = { line = glyph("─"), upright = glyph("│"), corner = glyph("·"), x = glyph("x") },
    edges = {
      across = function()
        return { "line" }
      end,
      down = function()
        return { "upright" }
      end,
      dot = function()
        return { "corner" }
      end,
    },
  }))
  assert_eq({
    "·────·────·",
    "│ x  │ x  │",
    "│    │    │",
    "·────·────·",
    "│ x  │ x  │",
    "│    │    │",
    "·────·────·",
  }, rendered.lines)
end

function T.grid_edges_refuse_flip_and_blocks()
  local edges = {
    across = function()
      return { "-" }
    end,
    down = function()
      return { "|" }
    end,
    dot = function()
      return { "+" }
    end,
  }
  for _, extra in ipairs({ { flip = true }, { blocks = 1 } }) do
    local ok, err = pcall(small_grid, vim.tbl_extend("force", { edges = edges }, extra))
    assert(not ok and err:find("edges draw neither flipped nor in blocks", 1, true), err)
  end
end

function T.key_hints_group_adjacent_hints_that_share_a_label()
  local function hints(spec)
    return vim.tbl_map(function(pair)
      return { key = pair[1], label = pair[2] }
    end, spec)
  end
  for _, case in ipairs({
    {
      name = "singletons",
      hints = hints({ { "m", "move" }, { "q", "close" } }),
      want = "m move  q close",
    },
    {
      name = "a gap splits the run",
      hints = hints({ { "1", "drop" }, { "3", "drop" }, { "4", "drop" }, { "5", "drop" } }),
      want = "1,3-5 drop",
    },
    {
      name = "letters run together and never form a range",
      hints = hints({ { "h", "move" }, { "j", "move" }, { "k", "move" }, { "l", "move" } }),
      want = "hjkl move",
    },
    {
      name = "a letter beside a longer key keeps the comma",
      hints = hints({ { "h", "left" }, { "<Left>", "left" } }),
      want = "h,<Left> left",
    },
    {
      name = "multi-character keys never form a range",
      hints = hints({ { "]g", "next" }, { "]h", "next" } }),
      want = "]g,]h next",
    },
    {
      name = "another label between splits the group",
      hints = hints({ { "1", "drop" }, { "R", "resign" }, { "2", "drop" } }),
      want = "1 drop  R resign  2 drop",
    },
  }) do
    assert_eq(
      { case.name, case.want },
      { case.name, compose.render(key_hints(case.hints)).lines[1] }
    )
  end
end

function T.key_hints_wrap_whole_pairs_to_the_width()
  local hints = {
    { key = "R", label = "resign" },
    { key = "D", label = "offer draw" },
    { key = "q", label = "close" },
  }
  for _, case in ipairs({
    { width = nil, want = { "R resign  D offer draw  q close" } },
    { width = 31, want = { "R resign  D offer draw  q close" } },
    { width = 30, want = { "R resign  D offer draw", "q close" } },
    { width = 5, want = { "R resign", "D offer draw", "q close" } },
  }) do
    local rendered = compose.render(key_hints(hints, case.width))
    assert_eq({ case.width, case.want }, { case.width, rendered.lines })
  end
  local wrapped = compose.render(key_hints(hints, 30))
  assert_eq({ "GamesKey", "GamesLabel" }, helpers.hls_on_row(wrapped, 1))
end

function T.movelist_offset_numbers_from_the_first_place_and_marks_the_skipped_ones()
  local list = function(offset)
    return compose.render(movelist({
      items = { "Bxg3", "Rxe7", "Qd1+" },
      columns = 2,
      numbered = true,
      offset = offset,
    })).lines
  end
  assert_eq({ "12. ...  Bxg3", "13. Rxe7 Qd1+" }, list(23))
  assert_eq({ " 9. Bxg3 Rxe7", "10. Qd1+" }, list(16))
  assert_eq({ "1. Bxg3 Rxe7", "2. Qd1+" }, list(nil))
end

function T.kv_panel_draws_keys_as_keys_and_values_as_labels()
  local rendered = compose.render(kv.panel({ { "K", "king" }, { "Q", "queen" } }))
  assert_eq({ "K king", "Q queen" }, rendered.lines)
  assert_eq({ "GamesKey", "GamesLabel" }, helpers.hls_on_row(rendered, 1))
end

function T.movelist_panel_keeps_its_least_width_until_the_list_needs_more_and_shows_the_last_lines()
  local short = movelist.panel({ items = { "e4", "e5", "Nf3" }, columns = 2, min_width = 12 })
  assert_eq(12, short.width)
  assert_eq({ "2. Nf3" }, compose.render(short.node(1)).lines)

  local long = movelist.panel({ items = { "Qxf7+", "Kxf7" }, columns = 2, min_width = 4 })
  assert_eq(#"1. Qxf7+ Kxf7", long.width)
end

function T.grid_indexes_items_by_cell_and_marks_the_cells_hl_of_picks()
  local items =
    { { r = 1, c = 2, name = "a" }, { r = 2, c = 1, name = "b" }, { r = 1, c = 2, name = "c" } }
  local by_cell = grid.cell_index(items, function(item)
    return item.r
  end, function(item)
    return item.c
  end)
  assert_eq("c", by_cell[1][2].name)
  assert_eq("b", by_cell[2][1].name)

  local marks = grid.marks_where({ { 0, 1 }, { 1, 0 } }, function(value)
    return value == 1 and "GamesLegal" or nil
  end)
  assert_eq({ grid.mark(1, 2, "GamesLegal"), grid.mark(2, 1, "GamesLegal") }, marks)
end

return T
