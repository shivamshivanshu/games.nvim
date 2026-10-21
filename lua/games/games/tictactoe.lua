local board = require("games.ui.widgets.board")
local players = require("games.core.players")
local theme = require("games.ui.theme")

---@alias games.TicTacToeSeat "x"|"o"

---@class games.TicTacToeView
---@field cells (games.TicTacToeSeat|"")[] each cell's owner in phone order, 1 at the top-left; "" when empty
---@field last? integer the cell placed last
---@field winning_lines integer[][] the cells of each completed line
---@field legal_cells integer[] the viewer's own: empty unless it is to move

---@class games.TicTacToeUpdate: games.GameUpdate
---@field view games.TicTacToeView

---@class games.TicTacToePlacedEvent: games.Event
---@field kind "placed"
---@field by games.TicTacToeSeat
---@field cell integer

local SIDE = 3
local CELLS = SIDE * SIDE
local CELL_WIDTH = 3
local PLACE_KIND = "place"
local PLACE_LABEL = "place"
local SEAT_KEYS = { "x", "o" }

local EMPTY_HL = "GamesMuted"
local LAST_MOVE_HL = "GamesLastMove"
local WIN_HL = "GamesWin"

---@param row integer
---@param col integer
---@return integer
local function cell_at(row, col)
  return (row - 1) * SIDE + col
end

---@param cell integer
---@param hl string
---@return games.GridMark
local function mark(cell, hl)
  return { row = math.floor((cell - 1) / SIDE) + 1, col = (cell - 1) % SIDE + 1, hl = hl }
end

---An empty cell draws its own number, the key that takes it.
---@param update games.TicTacToeUpdate
---@param cell integer
---@return games.GridCell
local function cell_for(update, cell)
  local owner = update.view.cells[cell]
  if owner == "" then
    return { tostring(cell), EMPTY_HL }
  end
  return { owner, theme.seat_hl(assert(players.seat_index(update, owner))) }
end

---@param view games.TicTacToeView
---@return games.GridMark[] the last move, then the winning lines over it
local function marks_for(view)
  local marks = {}
  if view.last then
    marks[1] = mark(view.last, LAST_MOVE_HL)
  end
  for _, line in ipairs(view.winning_lines) do
    for _, cell in ipairs(line) do
      marks[#marks + 1] = mark(cell, WIN_HL)
    end
  end
  return marks
end

local M = {}

---@type string[] keys every tictactoe asset pack draws: the seats' marks and each empty cell's number
M.asset_keys =
  vim.list_extend(vim.deepcopy(SEAT_KEYS), vim.tbl_map(tostring, vim.fn.range(1, CELLS)))

M.glyph_width = CELL_WIDTH

---@param update games.TicTacToeUpdate
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  return board({
    grid = {
      rows = SIDE,
      cols = SIDE,
      shade = "plain",
      blocks = 1,
      glyphs = ctx.glyphs,
      cell_width = CELL_WIDTH,
      cell = function(r, c)
        return cell_for(update, cell_at(r, c))
      end,
      marks = marks_for(update.view),
    },
    art = assert(ctx.art, "tictactoe draws from an asset pack"),
  }, ctx)
end

---@type games.GameAction[]
M.actions = vim.tbl_map(function(cell)
  return {
    key = tostring(cell),
    label = PLACE_LABEL,
    action = { kind = PLACE_KIND, cell = tostring(cell) },
    available = function(update)
      return vim.tbl_contains(update.view.legal_cells, cell)
    end,
  }
end, vim.fn.range(1, CELLS))

---@type table<games.EventKind, games.Describer>
M.describers = {
  placed = function(event, talk)
    ---@cast event games.TicTacToePlacedEvent
    return ("%s took cell %d"):format(talk.name(event.by), event.cell)
  end,
}

return M
