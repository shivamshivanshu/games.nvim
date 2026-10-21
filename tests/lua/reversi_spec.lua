local describe = require("games.core.describe")
local helpers = require("helpers")
local reversi = require("games.games.reversi")
local shell = require("games.ui.shell")

local assert_eq = helpers.assert_eq

local CATALOGUE = { place = { kind = "place", description = "", input = { field = "cell" } } }
---Lines above the board at cell height 1: two players, the status row and a blank.
local BOARD_TOP = 4
---Display columns before the first cell: the row label and its gap.
local BOARD_LEFT = 2

---@param glyphs games.Glyphs
---@param extra? table
---@return games.ViewOptions
local function view_options(glyphs, extra)
  return helpers.view_options("reversi", reversi, glyphs, extra)
end

---@param name string e.g. "d3"
---@return games.DiscCell
local function cell(name)
  return { row = tonumber(name:sub(2)), column = name:byte(1) - ("a"):byte() + 1 }
end

---The start position, black (alice) to move: the view black sees, with its four moves.
---@param overrides? table
---@return games.ReversiUpdate
local function start(overrides)
  local cells = {}
  for r = 1, 8 do
    cells[r] = { "", "", "", "", "", "", "", "" }
  end
  cells[4][4], cells[4][5], cells[5][4], cells[5][5] = "white", "black", "black", "white"
  return helpers.game_update("reversi", {
    players = { { user = "alice", seat = "black" }, { user = "bob", seat = "white" } },
    to_act = { "alice" },
    controls = { "resign", "offer_draw", "abort" },
    view = {
      cells = cells,
      legal_cells = vim.tbl_map(cell, { "d3", "c4", "f5", "e6" }),
      discs = { black = 2, white = 2 },
    },
  }, overrides)
end

---@param update games.ReversiUpdate
---@param me games.UserID
---@return games.Rendered
local function render(update, me)
  return (shell.render(update, reversi, view_options("ascii"), me, 0, nil, CATALOGUE))
end

---The highlight stack drawn on the ascii cell named name; none on a plain empty cell.
---@param rendered games.Rendered
---@param name string
---@return string[]
local function cell_hl(rendered, name)
  local at = cell(name)
  return helpers.hl_stack_at(rendered, BOARD_TOP + at.row - 1, BOARD_LEFT + (at.column - 1) * 2)
end

---The note beside board line r, such as "last h8"; nil when there is none.
---@param rendered games.Rendered
---@param r integer
---@return string|nil
local function note(rendered, r)
  return rendered.lines[BOARD_TOP + r]:match("   (%l+ [%w%-]+)%s*$")
end

---@param update games.GameUpdate
---@param event games.Event
---@return string what the event reads as in update
local function described(update, event)
  return describe.events(vim.tbl_extend("force", update, { events = { event } }))
end

local T = {}

function T.the_movers_legal_cells_carry_a_marker_glyph_and_tint()
  local mover = render(start(), "alice")
  assert_eq(
    { "1 . . . . . . . .", "3 . . . + . . . .", "4 . . + O X . . .", "6 . . . . + . . ." },
    vim.tbl_map(function(r)
      return mover.lines[BOARD_TOP + r]:sub(1, 17)
    end, { 1, 3, 4, 6 })
  )
  for _, name in ipairs({ "d3", "c4", "f5", "e6" }) do
    assert(vim.tbl_contains(cell_hl(mover, name), "GamesMovable"), "legal " .. name)
  end
  assert(not vim.tbl_contains(cell_hl(mover, "a1"), "GamesMovable"), "a1 is no move")

  local waiting = render(start({ view = { legal_cells = {} } }), "bob")
  assert(not waiting.lines[BOARD_TOP + 3]:find("+", 1, true), "the waiting seat sees no markers")
end

function T.each_header_line_counts_its_players_discs()
  local view = { discs = { black = 1, white = 12 } }
  local lines = render(start({ view = view }), "alice").lines
  assert(lines[1]:find("alice  black  1 disc ", 1, true), lines[1])
  assert(lines[2]:find("bob    white  12 discs", 1, true), lines[2])
end

function T.the_last_disc_is_tinted_and_named()
  local view = vim.deepcopy(start().view)
  view.cells[3][4], view.cells[4][4] = "black", "black"
  view.last = cell("d3")
  local rendered = render(start({ view = view, to_act = { "bob" } }), "bob")
  assert(vim.tbl_contains(cell_hl(rendered, "d3"), "GamesLastMove"))
  assert(vim.tbl_contains(cell_hl(rendered, "d3"), "GamesPieceBlack"))
  assert_eq("last d3", note(rendered, 1))
end

function T.m_types_a_cell()
  local lines = render(start(), "alice").lines
  assert_eq(
    "R resign  D offer draw  A abort  m place, e.g. d3  ]g next game  q close",
    lines[#lines]
  )
end

function T.events_name_the_move_and_the_automatic_pass()
  local placed = { kind = "placed", by = "black", cell = cell("d3"), flipped = { cell("d4") } }
  assert_eq("alice placed d3, flipping 1", described(start(), placed))
  assert_eq("bob has no move and passes", described(start(), { kind = "passed", by = "white" }))
end

return T
