local gomoku = require("games.games.gomoku")
local helpers = require("helpers")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local SIZE = 15
local CATALOGUE = helpers.catalogued("gomoku")
---Lines above the board at cell height 1: two players, the status row and a blank.
local BOARD_TOP = 4
---Display columns before the first cell: the widest row label, "15", and its gap.
local BOARD_LEFT = 3

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("gomoku", gomoku, glyphs, extra)
end

---@param name string e.g. "h8"
---@return games.Cell
local function cell(name)
  return { row = tonumber(name:sub(2)), col = name:byte(1) - ("a"):byte() + 1 }
end

---alice (black) has h8 to k8, bob (white) a1 to d1; bob is to move.
---@param overrides? table
---@return games.GomokuUpdate
local function update_with(overrides)
  local cells = {}
  for r = 1, SIZE do
    cells[r] = {}
    for c = 1, SIZE do
      cells[r][c] = ""
    end
  end
  for c = 8, 11 do
    cells[8][c] = "black"
  end
  for c = 1, 4 do
    cells[1][c] = "white"
  end
  return helpers.game_update("gomoku", {
    seq = 8,
    players = { { user = "alice", seat = "black" }, { user = "bob", seat = "white" } },
    to_act = { "alice" },
    controls = { "resign", "offer_draw" },
    view = { cells = cells, last = cell("d1"), winning_lines = {}, legal_cells = {} },
  }, overrides)
end

---alice's l8 completes h8 to l8.
---@return games.GomokuUpdate
local function won()
  local view = vim.deepcopy(update_with().view)
  view.cells[8][12] = "black"
  view.last = cell("l8")
  view.winning_lines = { vim.tbl_map(cell, { "h8", "i8", "j8", "k8", "l8" }) }
  return update_with({
    view = view,
    to_act = {},
    result = { kind = "decisive", winners = { "alice" }, reason = "five_in_a_row" },
  })
end

---@param update games.GomokuUpdate
---@param glyphs? games.Glyphs
---@return games.Rendered
local function render(update, glyphs)
  return (shell.render(update, gomoku, view_options(glyphs or "ascii"), "alice", 0, nil, CATALOGUE))
end

---The highlight stack drawn on the ascii cell named name; none on a plain empty cell.
---@param rendered games.Rendered
---@param name string
---@return string[]
local function cell_hl(rendered, name)
  local at = cell(name)
  return helpers.hl_stack_at(rendered, BOARD_TOP + at.row - 1, BOARD_LEFT + (at.col - 1) * 2)
end

---The note beside board line r, such as "last h8"; nil when there is none.
---@param rendered games.Rendered
---@param r integer
---@return string|nil
local function note(rendered, r)
  return rendered.lines[BOARD_TOP + r]:match("   (%l+ [%w%-]+)%s*$")
end

local T = {}

function T.board_letters_the_columns_and_numbers_the_rows_from_the_top_in_both_glyph_modes()
  for _, case in ipairs({
    {
      glyphs = "unicode",
      row1 = "1  ○ ○ ○ ○ · · · · · · · · · · ·",
      row8 = "8  · · · · · · · ● ● ● ● · · · ·",
    },
    {
      glyphs = "ascii",
      row1 = "1  O O O O . . . . . . . . . . .",
      row8 = "8  . . . . . . . X X X X . . . .",
    },
  }) do
    local lines = render(update_with(), case.glyphs).lines
    local function board_line(r)
      return lines[BOARD_TOP + r]:sub(1, #case.row1)
    end
    assert_eq({ case.glyphs, case.row1, case.row8 }, { case.glyphs, board_line(1), board_line(8) })
    assert_eq("   a b c d e f g h i j k l m n o", (lines[BOARD_TOP + SIZE + 1]:gsub("%s+$", "")))
  end
end

function T.stones_take_piece_colours_and_the_last_stone_and_a_win_are_marked_and_named()
  local ongoing = render(update_with())
  assert(vim.tbl_contains(cell_hl(ongoing, "h8"), "GamesPieceBlack"), "black stone")
  assert(vim.tbl_contains(cell_hl(ongoing, "a1"), "GamesPieceWhite"), "white stone")
  assert(vim.tbl_contains(cell_hl(ongoing, "d1"), "GamesLastMove"), "last stone tinted")
  assert_eq("last d1", note(ongoing, 1))

  local finished = render(won())
  for _, name in ipairs({ "h8", "i8", "j8", "k8", "l8" }) do
    assert(vim.tbl_contains(cell_hl(finished, name), "GamesWin"), "winning " .. name)
  end
  assert(not vim.tbl_contains(cell_hl(finished, "a1"), "GamesWin"), "white's stone is off the line")
  assert_eq({ "last l8", "line h8-l8" }, { note(finished, 1), note(finished, 2) })
end

function T.no_cell_is_marked_legal_though_the_mover_may_play_any_empty_one()
  local view =
    vim.tbl_extend("force", update_with().view, { legal_cells = { cell("a2"), cell("o15") } })
  local rendered = render(update_with({ view = view }))
  assert(not vim.tbl_contains(cell_hl(rendered, "a2"), "GamesMovable"))
  assert(not rendered.lines[BOARD_TOP + 2]:find("+", 1, true), "no legal marker glyph")
end

function T.m_types_a_cell_while_the_game_is_on()
  local function footer(update)
    local lines = render(update).lines
    return lines[#lines]
  end
  assert_eq(
    "R resign  D offer draw  m place, e.g. h8  ]g next game  q close",
    footer(update_with())
  )
  assert_eq("]g next game  q close", footer(vim.tbl_extend("force", won(), { controls = {} })))
end

function T.events_name_the_player_and_cell()
  local event = { kind = "placed", by = "white", cell = cell("o15") }
  assert_eq("bob placed o15", helpers.described(update_with(), event))
end

return T
