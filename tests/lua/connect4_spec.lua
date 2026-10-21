local connect4 = require("games.games.connect4")
local helpers = require("helpers")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local CATALOGUE = { drop = true }
local ALL_COLUMNS = { 1, 2, 3, 4, 5, 6, 7 }
---Lines above the board at cell height 1: two players, the status row and a blank.
local BOARD_TOP = 4
---Display columns before the first cell: the blank row label and its gap.
local BOARD_LEFT = 1

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("connect4", connect4, glyphs, extra)
end

---alice (red) has three along the bottom under bob's three; bob is to move.
---@param overrides? table
---@return games.Connect4Update
local function update_with(overrides)
  local cells = {}
  for r = 1, 4 do
    cells[r] = { "", "", "", "", "", "", "" }
  end
  cells[5] = { "yellow", "yellow", "yellow", "", "", "", "" }
  cells[6] = { "red", "red", "red", "", "", "", "" }
  return helpers.game_update("connect4", {
    seq = 6,
    players = { { user = "alice", seat = "red" }, { user = "bob", seat = "yellow" } },
    to_act = { "bob" },
    controls = { "resign", "offer_draw" },
    view = {
      cells = cells,
      last = { row = 5, column = 3 },
      winning_lines = {},
      legal_columns = ALL_COLUMNS,
    },
  }, overrides)
end

---@param update games.Connect4Update
---@param me games.UserID
---@return games.Rendered
local function render(update, me)
  return (shell.render(update, connect4, view_options("ascii"), me, 0, nil, CATALOGUE))
end

---The highlight stack drawn on the ascii cell at row r, column c.
---@param rendered games.Rendered
---@param r integer
---@param c integer
---@return string[]
local function cell_hl(rendered, r, c)
  return helpers.hl_stack_at(rendered, BOARD_TOP + r - 1, BOARD_LEFT + (c - 1) * 2)
end

local T = {}

function T.board_draws_each_seats_glyph_with_column_labels_in_both_glyph_modes()
  for _, case in ipairs({
    { glyphs = "unicode", bottom = { " ○ ○ ○ · · · ·", " ● ● ● · · · ·" } },
    { glyphs = "ascii", bottom = { " O O O . . . .", " X X X . . . ." } },
  }) do
    local rendered =
      shell.render(update_with(), connect4, view_options(case.glyphs), "bob", 0, nil, CATALOGUE)
    local board = vim.list_slice(rendered.lines, BOARD_TOP + 1, BOARD_TOP + 7)
    assert_eq({ case.glyphs, case.bottom }, { case.glyphs, vim.list_slice(board, 5, 6) })
    assert_eq(" 1 2 3 4 5 6 7", board[7])
  end
end

function T.footer_offers_the_open_columns_only_to_the_seat_to_move()
  local function footer(update, me)
    local lines = render(update, me).lines
    return lines[#lines]
  end
  local column_four_full = update_with({
    view = { legal_columns = { 1, 2, 3, 5, 6, 7 } },
  })
  local waiting = update_with({ view = { legal_columns = {} } })

  assert_eq("R resign  D offer draw  1-7 drop  ]g next game  q close", footer(update_with(), "bob"))
  assert_eq(
    "R resign  D offer draw  1-3,5-7 drop  ]g next game  q close",
    footer(column_four_full, "bob")
  )
  assert_eq("R resign  D offer draw  ]g next game  q close", footer(waiting, "alice"))
end

function T.discs_take_their_players_header_colour_and_marks_tint_last_and_winning_cells()
  local winning = {
    { row = 6, column = 1 },
    { row = 6, column = 2 },
    { row = 6, column = 3 },
    { row = 6, column = 4 },
  }
  local view = vim.deepcopy(update_with().view)
  view.cells[6][4] = "red"
  view.last, view.winning_lines, view.legal_columns = { row = 6, column = 4 }, { winning }, {}
  local won = update_with({
    view = view,
    to_act = {},
    result = { kind = "decisive", winners = { "alice" }, reason = "four_in_a_row" },
  })

  local ongoing = render(update_with(), "bob")
  assert(vim.tbl_contains(cell_hl(ongoing, 6, 1), "GamesSeat1"), "red is alice's, the first player")
  assert(vim.tbl_contains(cell_hl(ongoing, 5, 1), "GamesSeat2"), "yellow is bob's")
  assert(vim.tbl_contains(cell_hl(ongoing, 5, 3), "GamesLastMove"), "the last disc is tinted")

  local finished = render(won, "bob")
  for c = 1, 4 do
    assert(vim.tbl_contains(cell_hl(finished, 6, c), "GamesWin"), ("winning cell 6,%d"):format(c))
  end
  assert(
    not vim.tbl_contains(cell_hl(finished, 5, 1), "GamesWin"),
    "yellow's disc is not on the line"
  )
end

function T.a_drop_key_sends_its_column_as_text()
  local four = vim.iter(connect4.actions):find(function(entry)
    return entry.key == "4"
  end)
  assert_eq({ kind = "drop", column = "4" }, four.action)
end

function T.events_name_the_player_and_column()
  local event = { kind = "dropped", by = "yellow", row = 5, column = 3 }
  assert_eq("bob dropped in column 3", helpers.described(update_with(), event))
end

return T
