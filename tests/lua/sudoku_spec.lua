local helpers = require("helpers")
local shell = require("games.ui.shell")
local sudoku = require("games.games.sudoku")

local assert_eq = helpers.assert_eq

local CATALOGUED = { set = true }
local PUZZLE = "530070000600195000098000060800060003400803001700020006060000280000419005000080079"
local SOLUTION = "534678912672195348198342567859761423426853791713924856961537284287419635345286179"
local FIRST_BOARD_ROW = 3
local BOARD_LINES = 12
local FOOTER_ROW = 16

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("sudoku", sudoku, glyphs, extra)
end

---@param digits string
---@return integer[]
local function digits_of(digits)
  return vim.tbl_map(tonumber, vim.split(digits, ""))
end

---alice's puzzle with a 5 written at row 1, column 3, which repeats the given 5 at row 1, column 1.
---@param overrides? table
---@return games.SudokuUpdate
local function update(overrides)
  local grid = digits_of(PUZZLE)
  local given = vim.tbl_map(function(digit)
    return digit ~= 0
  end, grid)
  local conflicts = vim.tbl_map(function()
    return false
  end, grid)
  grid[3], conflicts[1], conflicts[3] = 5, true, true
  return helpers.game_update("sudoku", {
    seq = 1,
    options = { difficulty = "easy" },
    players = { { user = "alice", seat = "solver" } },
    to_act = { "alice" },
    controls = { "resign", "abort" },
    view = { grid = grid, given = given, conflicts = conflicts, solution = {} },
  }, overrides)
end

---@return games.SudokuUpdate alice gave up with the wrong 5 still in place
local function gave_up()
  local over = update({
    to_act = {},
    controls = {},
    result = { kind = "decisive", winners = {}, reason = "resign" },
  })
  over.view.solution = digits_of(SOLUTION)
  return over
end

local T = {}

function T.board_draws_the_grid_in_boxes_with_labels_and_frames_conflicts_without_colour()
  for _, case in ipairs({
    {
      glyphs = "unicode",
      board = {
        "1 [5]  3  [5]│ ·   7   · │ ·   ·   · ",
        "2  6   ·   · │ 1   9   5 │ ·   ·   · ",
        "3  ·   9   8 │ ·   ·   · │ ·   6   · ",
        "  ───────────┼───────────┼───────────",
        "4  8   ·   · │ ·   6   · │ ·   ·   3 ",
        "5  4   ·   · │ 8   ·   3 │ ·   ·   1 ",
        "6  7   ·   · │ ·   2   · │ ·   ·   6 ",
        "  ───────────┼───────────┼───────────",
        "7  ·   6   · │ ·   ·   · │ 2   8   · ",
        "8  ·   ·   · │ 4   1   9 │ ·   ·   5 ",
        "9  ·   ·   · │ ·   8   · │ ·   7   9 ",
        "   1   2   3   4   5   6   7   8   9 ",
      },
    },
    {
      glyphs = "ascii",
      board = {
        "1 [5]  3  [5]| .   7   . | .   .   . ",
        "2  6   .   . | 1   9   5 | .   .   . ",
        "3  .   9   8 | .   .   . | .   6   . ",
        "  -----------+-----------+-----------",
        "4  8   .   . | .   6   . | .   .   3 ",
        "5  4   .   . | 8   .   3 | .   .   1 ",
        "6  7   .   . | .   2   . | .   .   6 ",
        "  -----------+-----------+-----------",
        "7  .   6   . | .   .   . | 2   8   . ",
        "8  .   .   . | 4   1   9 | .   .   5 ",
        "9  .   .   . | .   8   . | .   7   9 ",
        "   1   2   3   4   5   6   7   8   9 ",
      },
    },
  }) do
    local rendered =
      shell.render(update(), sudoku, view_options(case.glyphs), "alice", 0, nil, CATALOGUED)
    assert_eq(
      case.board,
      vim.list_slice(rendered.lines, FIRST_BOARD_ROW + 1, FIRST_BOARD_ROW + BOARD_LINES)
    )
  end
end

function T.the_set_key_needs_the_catalogue_and_an_unfinished_game()
  for _, case in ipairs({
    {
      state = update(),
      catalogued = CATALOGUED,
      footer = "R give up  A abort  m set, e.g. 53 7  ]g next game  q close",
    },
    { state = update(), catalogued = nil, footer = "R give up  A abort  ]g next game  q close" },
    { state = gave_up(), catalogued = CATALOGUED, footer = "]g next game  q close" },
  }) do
    local rendered, _, bindings =
      shell.render(case.state, sudoku, view_options("unicode"), "alice", 0, nil, case.catalogued)
    assert_eq(case.footer, rendered.lines[FOOTER_ROW + 1])
    local set = vim.iter(bindings):find(function(binding)
      return binding.key == "m"
    end)
    assert_eq(case.footer:find(" m ") ~= nil, set ~= nil)
  end
end

function T.input_sends_the_typed_text_for_the_server_to_parse()
  assert_eq("set", sudoku.input.kind)
  assert_eq({ kind = "set", text = "53 7" }, sudoku.input.parse("53 7"))
end

function T.givens_are_bold_and_muted_entries_take_the_seat_colour_and_conflicts_are_tinted()
  local rendered =
    shell.render(update(), sudoku, view_options("unicode"), "alice", 0, nil, CATALOGUED)
  assert_eq(
    { { "GamesCheck", "GamesGiven" }, "GamesGiven", { "GamesCheck", "GamesSeat1" } },
    vim.list_slice(helpers.hls_on_row(rendered, FIRST_BOARD_ROW), 1, 3)
  )
end

function T.after_giving_up_the_solution_replaces_wrong_and_empty_cells_and_conflicts_clear()
  local rendered =
    shell.render(gave_up(), sudoku, view_options("unicode"), "alice", 0, nil, CATALOGUED)
  assert_eq(
    { "1  5   3  (4)│(6)  7  (8)│(9) (1) (2)", "2  6  (7) (2)│ 1   9   5 │(3) (4) (8)" },
    vim.list_slice(rendered.lines, FIRST_BOARD_ROW + 1, FIRST_BOARD_ROW + 2)
  )
  assert_eq(
    { "GamesGiven", "GamesGiven", { "GamesMiss", "GamesMuted" } },
    vim.list_slice(helpers.hls_on_row(rendered, FIRST_BOARD_ROW), 1, 3)
  )
end

function T.describe_event_names_the_cell_and_digit()
  assert_eq(
    "alice set row 5, column 3 to 7",
    sudoku.describe_event(
      { kind = "cell_set", by = "solver", row = 5, col = 3, digit = 7 },
      update()
    )
  )
  assert_eq(
    "alice cleared row 5, column 3",
    sudoku.describe_event(
      { kind = "cell_set", by = "solver", row = 5, col = 3, digit = 0 },
      update()
    )
  )
  assert_eq(nil, sudoku.describe_event({ kind = "other", by = "solver" }, update()))
end

return T
