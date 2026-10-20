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

---Each seat's colour, the one the header gives its player.
---@param update games.Connect4Update
---@return table<games.Connect4Seat, string>
local function seat_colours(update)
  local colours = {}
  for i, player in ipairs(update.players) do
    colours[player.seat] = theme.seat_hl(i)
  end
  return colours
end

---@param seat games.Connect4Seat|""
---@param colours table<games.Connect4Seat, string>
---@return games.GridCell
local function cell_for(seat, colours)
  if seat == "" then
    return { EMPTY_KEY }
  end
  return { seat, colours[seat] }
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
  local colours = seat_colours(update)
  return board({
    grid = {
      rows = ROWS,
      cols = COLUMNS,
      shade = "plain",
      labels = { cols = COLUMN_LABELS, rows = ROW_LABELS },
      cell = function(r, c)
        return cell_for(view.cells[r][c], colours)
      end,
      marks = marks_for(view),
    },
    art = ctx.art,
  }, ctx)
end

---@type games.GameAction[]
M.actions = vim.iter(vim.fn.range(1, COLUMNS)):map(drop_action):totable()

---@param event games.Event
---@param update games.Connect4Update
---@return string|nil
function M.describe_event(event, update)
  if event.kind ~= "dropped" then
    return nil
  end
  ---@cast event games.Connect4DroppedEvent
  local who = players.user_of_seat(update, event.by) or event.by
  return ("%s dropped in column %d"):format(who, event.column)
end

return M
