local actions = require("games.ui.actions")
local board = require("games.ui.widgets.board")
local discs = require("games.ui.discs")
local theme = require("games.ui.theme")

---@alias games.Connect4Seat "red"|"yellow"

---@class games.Connect4View: games.DiscLast
---@field cells (games.Connect4Seat|"")[][] rows from the top; "" for an empty cell
---@field last? games.Cell where the latest disc came to rest
---@field winning_lines games.Cell[][] every winning run through the last disc, whole
---@field legal_columns integer[] the viewer's own: open columns, empty unless it is to move

---@class games.Connect4Update: games.GameUpdate
---@field view games.Connect4View

---@class games.Connect4DroppedEvent: games.Event
---@field kind "dropped"
---@field by games.Connect4Seat
---@field cell games.Cell where the disc came to rest

local ROWS = 6
local COLUMNS = 7
local COLUMN_LABELS = "1234567"
local EMPTY_KEY = "empty"
local DROP_KIND = "drop"

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
  return { seat, theme.seat_hl_of(update, seat) }
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
      labels = { cols = COLUMN_LABELS, rows = ROW_LABELS },
      cell = function(r, c)
        return cell_for(update, view.cells[r][c])
      end,
      marks = discs.marks(view, { winning_lines = view.winning_lines }),
    },
  }, ctx)
end

---@type games.GameAction[]
M.actions = actions.per_value({
  kind = DROP_KIND,
  field = "column",
  values = vim.fn.range(1, COLUMNS),
  legal = function(view)
    return view.legal_columns
  end,
})

---@type table<games.EventKind, games.Describer>
M.describers = {
  dropped = function(event, talk)
    ---@cast event games.Connect4DroppedEvent
    return ("%s dropped in column %d"):format(talk.name(event.by), event.cell.col)
  end,
}

return M
