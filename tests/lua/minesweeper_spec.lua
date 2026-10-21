local helpers = require("helpers")
local minesweeper = require("games.games.minesweeper")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local CELL_INPUT = { input = { field = "cell" } }
local CATALOGUED = { reveal = CELL_INPUT, flag = CELL_INPUT, chord = CELL_INPUT }
local FIRST_BOARD_ROW = 3
local CELL_CODES = {
  ["#"] = "hidden",
  F = "flag",
  ["*"] = "mine",
  M = "flagged_mine",
  X = "exploded",
  ["!"] = "wrong_flag",
}

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("minesweeper", minesweeper, glyphs, extra)
end

---@param rows string[] "#" hidden, "F" flag, a digit revealed, and "*", "M", "X", "!" once over
---@return games.MinesweeperCell[][]
local function cells_of(rows)
  return vim.tbl_map(function(row)
    return vim.tbl_map(function(code)
      return CELL_CODES[code] or code
    end, vim.split(row, ""))
  end, rows)
end

---alice's easy board after an opening reveal and a flag on a1's corner number.
---@param overrides? table
---@return games.MinesweeperUpdate
local function update(overrides)
  return helpers.game_update("minesweeper", {
    seq = 2,
    options = { difficulty = "easy" },
    players = { { user = "alice", seat = "solver" } },
    to_act = { "alice" },
    controls = { "resign" },
    view = {
      cells = cells_of({
        "001F1####",
        "00111####",
        "000001###",
        "000001###",
        "11112####",
        "#########",
        "#########",
        "#########",
        "#########",
      }),
      flags = 1,
      remaining = 9,
      chordable = { { row = 1, col = 5 } },
    },
  }, overrides)
end

---@return games.MinesweeperUpdate alice revealed the mine at a6 with a wrong flag on a4 and a right one on b7
local function lost()
  return update({
    to_act = {},
    controls = {},
    result = { kind = "decisive", winners = {}, reason = "mine" },
    view = {
      cells = cells_of({ "001!1X##", "00111#M*" }),
      flags = 1,
      remaining = 1,
      chordable = {},
    },
  })
end

---@param rendered games.Rendered
---@param needle string
---@return games.Hl[] every highlight on the line holding needle
local function hls_on_line_with(rendered, needle)
  for i, line in ipairs(rendered.lines) do
    if line:find(needle, 1, true) then
      return helpers.hls_on_row(rendered, i - 1)
    end
  end
  error(("%q not rendered"):format(needle))
end

local T = {}

function T.board_labels_rows_by_letter_and_columns_by_number_beside_the_flag_count()
  for _, case in ipairs({
    {
      glyphs = "unicode",
      board = {
        "a       1  ⚑  1  ░░ ░░ ░░ ░░   flags      1",
        "b       1  1  1  ░░ ░░ ░░ ░░   mines left 9",
        "c                1  ░░ ░░ ░░               ",
        "d                1  ░░ ░░ ░░               ",
        "e 1  1  1  1  2  ░░ ░░ ░░ ░░               ",
        "f ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░               ",
        "g ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░               ",
        "h ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░               ",
        "i ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░ ░░               ",
        "  1  2  3  4  5  6  7  8  9                ",
      },
    },
    {
      glyphs = "ascii",
      board = {
        "a       1  F  1  ## ## ## ##   flags      1",
        "b       1  1  1  ## ## ## ##   mines left 9",
        "c                1  ## ## ##               ",
        "d                1  ## ## ##               ",
        "e 1  1  1  1  2  ## ## ## ##               ",
        "f ## ## ## ## ## ## ## ## ##               ",
        "g ## ## ## ## ## ## ## ## ##               ",
        "h ## ## ## ## ## ## ## ## ##               ",
        "i ## ## ## ## ## ## ## ## ##               ",
        "  1  2  3  4  5  6  7  8  9                ",
      },
    },
  }) do
    local rendered =
      shell.render(update(), minesweeper, view_options(case.glyphs), "alice", 0, nil, CATALOGUED)
    assert_eq(case.board, vim.list_slice(rendered.lines, FIRST_BOARD_ROW + 1, FIRST_BOARD_ROW + 10))
  end
end

function T.a_hard_board_numbers_all_thirty_columns()
  local rows = vim.fn["repeat"]({ ("#"):rep(30) }, 16)
  local hard =
    update({ view = { cells = cells_of(rows), flags = 0, remaining = 99, chordable = {} } })
  local rendered =
    shell.render(hard, minesweeper, view_options("ascii"), "alice", 0, nil, CATALOGUED)
  local labels = rendered.lines[FIRST_BOARD_ROW + 17]
  assert(vim.startswith(labels, "  1  2  3"), labels)
  assert(vim.endswith(vim.trim(labels), "28 29 30"), labels)
  assert(vim.startswith(rendered.lines[FIRST_BOARD_ROW + 16], "p ## "))
end

function T.keys_follow_the_catalogue_the_chordable_cells_and_the_end()
  local function footer(state, catalogued)
    local rendered =
      shell.render(state, minesweeper, view_options("unicode"), "alice", 0, nil, catalogued)
    return rendered.lines[#rendered.lines]
  end
  local nothing_to_chord = update()
  nothing_to_chord.view = vim.tbl_extend("force", nothing_to_chord.view, { chordable = {} })
  assert_eq(
    "R give up  m reveal, e.g. c5  f flag  c chord  ]g next game  q close",
    footer(update(), CATALOGUED)
  )
  assert_eq(
    "R give up  m reveal, e.g. c5  f flag  ]g next game  q close",
    footer(nothing_to_chord, CATALOGUED)
  )
  assert_eq("]g next game  q close", footer(lost(), CATALOGUED))
end

function T.a_lost_board_tells_flagged_unflagged_and_exploded_mines_and_wrong_flags_by_glyph()
  for glyphs, want in pairs({
    unicode = {
      "a       1  ✗  1  ✸  ░░ ░░   flags      1",
      "b       1  1  1  ░░ ⚑✱ ✱    mines left 1",
    },
    ascii = {
      "a       1  Fx 1  *! ## ##   flags      1",
      "b       1  1  1  ## F* *    mines left 1",
    },
  }) do
    local rendered =
      shell.render(lost(), minesweeper, view_options(glyphs), "alice", 0, nil, CATALOGUED)
    assert_eq(want, vim.list_slice(rendered.lines, FIRST_BOARD_ROW + 1, FIRST_BOARD_ROW + 2))
  end
  local hls = hls_on_line_with(
    shell.render(lost(), minesweeper, view_options("ascii"), "alice", 0, nil, CATALOGUED),
    "*!"
  )
  assert_eq({
    "GamesCardRed",
    { "GamesCheck", "GamesCheckInk" },
    "GamesMuted",
    "GamesMuted",
    "GamesKey",
    "GamesLabel",
  }, hls)
end

function T.describe_event_names_the_cell_as_typed()
  local cases = {
    {
      { kind = "revealed", by = "solver", row = 3, col = 12, opened = 14 },
      "alice revealed c12, opening 14 cells",
    },
    {
      { kind = "revealed", by = "solver", row = 1, col = 1, opened = 1 },
      "alice revealed a1, opening 1 cell",
    },
    { { kind = "revealed", by = "solver", row = 2, col = 2, opened = 0 }, "alice revealed b2" },
    {
      { kind = "chorded", by = "solver", row = 4, col = 5, opened = 3 },
      "alice chorded d5, opening 3 cells",
    },
    { { kind = "flagged", by = "solver", row = 1, col = 2, flagged = true }, "alice flagged a2" },
    {
      { kind = "flagged", by = "solver", row = 1, col = 2, flagged = false },
      "alice unflagged a2",
    },
    { { kind = "other", by = "solver" }, nil },
  }
  for _, case in ipairs(cases) do
    assert_eq(case[2], minesweeper.describe_event(case[1], update()))
  end
end

return T
