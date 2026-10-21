local board = require("games.ui.widgets.board")
local players = require("games.core.players")
local theme = require("games.ui.theme")

---@alias games.Connect4Seat "red"|"yellow"

---@class games.Connect4Cell
---@field row integer 1 at the top
---@field column integer 1 at the left

---@class games.Connect4View
---@field cells (games.Connect4Seat|"")[][] rows from the top; "" for an empty cell
---@field last? games.Connect4Cell where the latest disc came to rest
---@field winning_lines games.Connect4Cell[][] every winning run through the last disc, whole
---@field legal_columns integer[] the viewer's own: open columns, empty unless it is to move

---@class games.Connect4Update: games.GameUpdate
---@field view games.Connect4View

---@class games.Connect4DroppedEvent: games.Event
---@field kind "dropped"
---@field by games.Connect4Seat
---@field row integer
---@field column integer

local ROWS = 6
local COLUMNS = 7
local COLUMN_LABELS = "1234567"
local EMPTY_KEY = "empty"
local DROP_KIND = "drop"
local WIN_HL = "GamesWin"
local LAST_MOVE_HL = "GamesLastMove"

---Blank row labels: rows are never typed, only columns are.
---@type string[]
local ROW_LABELS = vim.fn["repeat"]({ "" }, ROWS)

---A disc takes the colour the header gives its player.
---@param update games.Connect4Update
---@param seat games.Connect4Seat|""
---@return games.GridCell
local function cell_for(update, seat)
  if seat == "" then
    return { EMPTY_KEY }
  end
  return { seat, theme.seat_hl(assert(players.seat_index(update, seat))) }
end

---The last disc, then the winning lines over it; a later mark on a cell replaces an earlier one.
---@param view games.Connect4View
---@return games.GridMark[]
local function marks_for(view)
  local marks = {}
  if view.last then
    marks[1] = { row = view.last.row, col = view.last.column, hl = LAST_MOVE_HL }
  end
  for _, line in ipairs(view.winning_lines) do
    for _, cell in ipairs(line) do
      marks[#marks + 1] = { row = cell.row, col = cell.column, hl = WIN_HL }
    end
  end
  return marks
end

---@param column integer
---@return games.GameAction
local function drop_action(column)
  return {
    key = tostring(column),
    label = DROP_KIND,
    action = { kind = DROP_KIND, column = tostring(column) },
    available = function(update)
      return vim.tbl_contains(update.view.legal_columns, column)
    end,
  }
end

local M = {}

---@type string[] keys every connect4 asset pack draws
M.asset_keys = { "red", "yellow", EMPTY_KEY }

---@param update games.Connect4Update
---@param ctx games.LayoutCtx
---@return games.Node
function M.layout(update, ctx)
  local view = update.view
  return board({
    grid = {
      rows = ROWS,
      cols = COLUMNS,
      shade = "plain",
      labels = { cols = COLUMN_LABELS, rows = ROW_LABELS },
      cell = function(r, c)
        return cell_for(update, view.cells[r][c])
      end,
      marks = marks_for(view),
    },
    art = ctx.art,
  }, ctx)
end

---@type games.GameAction[]
M.actions = vim.iter(vim.fn.range(1, COLUMNS)):map(drop_action):totable()

---@type table<games.EventKind, games.Describer>
M.describers = {
  dropped = function(event, talk)
    ---@cast event games.Connect4DroppedEvent
    return ("%s dropped in column %d"):format(talk.name(event.by), event.column)
  end,
}

return M
