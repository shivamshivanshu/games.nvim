local helpers = require("helpers")
local shell = require("games.ui.shell")
local tictactoe = require("games.games.tictactoe")

local assert_eq = helpers.assert_eq

local CATALOGUED = helpers.catalogued("tictactoe")
local BOARD_ROWS = { 4, 8 }
local FOOTER_ROW = 10

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("tictactoe", tictactoe, glyphs, extra)
end

---alice (x) and bob (o) after x 1, o 4, x 2: bob to move, last move on 2.
---@param overrides? table
---@return games.TicTacToeUpdate
local function update(overrides)
  return helpers.game_update("tictactoe", {
    seq = 3,
    players = { { user = "alice", seat = "x" }, { user = "bob", seat = "o" } },
    to_act = { "bob" },
    controls = { "resign", "abort" },
    view = {
      cells = { "x", "x", "", "o", "", "", "", "", "" },
      last = { row = 1, col = 2 },
      winning_lines = {},
      legal_cells = { 3, 5, 6, 7, 8, 9 },
    },
  }, overrides)
end

local T = {}

function T.board_draws_marks_in_phone_order_and_numbers_the_free_cells()
  for _, case in ipairs({
    {
      glyphs = "unicode",
      board = {
        " ✕ │ ✕ │ 3 ",
        "───┼───┼───",
        " ○ │ 5 │ 6 ",
        "───┼───┼───",
        " 7 │ 8 │ 9 ",
      },
    },
    {
      glyphs = "ascii",
      board = { " X | X | 3 ", "---+---+---", " O | 5 | 6 ", "---+---+---", " 7 | 8 | 9 " },
    },
  }) do
    local rendered =
      shell.render(update(), tictactoe, view_options(case.glyphs), "bob", 0, nil, CATALOGUED)
    assert_eq(case.board, vim.list_slice(rendered.lines, BOARD_ROWS[1] + 1, BOARD_ROWS[2] + 1))
  end
end

function T.place_keys_are_the_movers_legal_cells_and_need_the_catalogue()
  for _, case in ipairs({
    {
      me = "bob",
      catalogued = CATALOGUED,
      footer = "R resign  A abort  3,5-9 place  ]g next game  q close",
    },
    { me = "bob", catalogued = nil, footer = "R resign  A abort  ]g next game  q close" },
    {
      me = "alice",
      catalogued = CATALOGUED,
      legal_cells = {},
      footer = "R resign  A abort  ]g next game  q close",
    },
  }) do
    local state = update()
    state.view.legal_cells = case.legal_cells or state.view.legal_cells
    local rendered, _, bindings =
      shell.render(state, tictactoe, view_options("unicode"), case.me, 0, nil, case.catalogued)
    assert_eq(case.footer, rendered.lines[FOOTER_ROW + 1])
    local place = vim.iter(bindings):find(function(binding)
      return binding.key == "5"
    end)
    assert_eq(case.footer:find("place") ~= nil, place ~= nil)
  end
end

function T.place_actions_send_the_cell_as_text()
  assert_eq(
    vim.tbl_map(function(n)
      return { key = tostring(n), action = { kind = "place", cell = tostring(n) } }
    end, vim.fn.range(1, 9)),
    vim.tbl_map(function(entry)
      return { key = entry.key, action = entry.action }
    end, tictactoe.actions)
  )
end

function T.marks_take_their_seats_colour_in_player_order_with_last_move_and_win_tints()
  local won = update({
    to_act = {},
    result = { kind = "decisive", winners = { "alice" }, reason = "three_in_a_row" },
  })
  won.view.winning_lines = { { { row = 1, col = 1 }, { row = 2, col = 2 }, { row = 3, col = 3 } } }
  local rendered = shell.render(won, tictactoe, view_options("unicode"), "bob", 0, nil, CATALOGUED)
  assert_eq({
    { "GamesWin", "GamesSeat1" },
    "GamesMuted",
    { "GamesLastMove", "GamesSeat1" },
    "GamesMuted",
    "GamesMuted",
  }, helpers.hls_on_row(rendered, BOARD_ROWS[1]))
  assert_eq(
    { "GamesSeat2", "GamesMuted", { "GamesWin", "GamesMuted" }, "GamesMuted", "GamesMuted" },
    helpers.hls_on_row(rendered, BOARD_ROWS[1] + 2)
  )

  local bob_first =
    update({ players = { { user = "bob", seat = "o" }, { user = "alice", seat = "x" } } })
  local swapped =
    shell.render(bob_first, tictactoe, view_options("unicode"), "bob", 0, nil, CATALOGUED)
  assert_eq(
    { "GamesSeat2", "GamesMuted" },
    vim.list_slice(helpers.hls_on_row(swapped, BOARD_ROWS[1]), 1, 2)
  )
end

function T.a_roomy_window_draws_the_marks_as_art()
  local rendered = shell.render(update(), tictactoe, view_options("unicode"), "bob", 0, {
    text_area = { w = 80, h = 24 },
  }, CATALOGUED)
  assert(
    vim.iter(rendered.lines):any(function(line)
      return line:find("▜█▖    ▗█▛", 1, true) ~= nil
    end),
    table.concat(rendered.lines, "\n")
  )
end

function T.events_name_the_player_and_cell()
  assert_eq("bob took cell 5", helpers.described(update(), { kind = "placed", by = "o", cell = 5 }))
end

return T
