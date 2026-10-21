---A cursor a board's keys move over a grid, lying across or down, for games that place pieces
---longer than a cell. It lives in the board's view options, so moving it only redraws the board.
local keys = require("games.ui.keys")

local M = {}

---@enum games.Orientation
M.Orientation = { ACROSS = "across", DOWN = "down" }

---@class games.GridPos
---@field row integer 1 at the top
---@field col integer 1 at the left

---@class games.GridCursor: games.GridPos
---@field orientation games.Orientation

---@class games.CursorBounds
---@field rows integer
---@field cols integer

---Bounds that follow the way the cursor lies, such as over the edges between a board's cells:
---one more row of them lies across than down, and one more column down than across.
---@class games.OrientedBounds
---@field across games.CursorBounds
---@field down games.CursorBounds

---Where the cursor may go: fixed, or read off the board's update for a board whose size is an option.
---@alias games.CursorReach games.CursorBounds|games.OrientedBounds|fun(update: games.GameUpdate): games.CursorBounds|games.OrientedBounds

local MOVE_LABEL = "move"
local ROTATE_KEY, ROTATE_LABEL = "r", "turn"

---@type games.GridCursor
local START = { row = 1, col = 1, orientation = M.Orientation.ACROSS }

---@type table<games.Orientation, games.Orientation>
local TURNED =
  { [M.Orientation.ACROSS] = M.Orientation.DOWN, [M.Orientation.DOWN] = M.Orientation.ACROSS }

---The cursor of a board's view options: the top left cell, lying across, until a key moves it.
---@param view games.ViewOptions
---@return games.GridCursor
function M.of(view)
  return view.cursor or START
end

---@param view games.ViewOptions
---@param cursor games.GridCursor
---@return games.ViewOptions
local function with_cursor(view, cursor)
  return vim.tbl_extend("force", view, { cursor = cursor })
end

---@param reach games.CursorReach
---@param orientation games.Orientation
---@param update games.GameUpdate
---@return games.CursorBounds those of a cursor lying orientation
local function bounds_lying(reach, orientation, update)
  local bounds = type(reach) == "function" and reach(update) or reach
  if bounds.rows then
    ---@cast bounds games.CursorBounds
    return bounds
  end
  ---@cast bounds games.OrientedBounds
  return bounds[orientation]
end

---@param row integer
---@param col integer
---@param orientation games.Orientation
---@param reach games.CursorReach
---@param update games.GameUpdate
---@return games.GridCursor at row and col, moved within the bounds of lying orientation
local function clamped(row, col, orientation, reach, update)
  local within = bounds_lying(reach, orientation, update)
  return {
    row = math.min(math.max(row, 1), within.rows),
    col = math.min(math.max(col, 1), within.cols),
    orientation = orientation,
  }
end

---Keys h, j, k and l moving the cursor one cell, stopping at the edges of reach.
---@param reach games.CursorReach
---@param available? fun(update: games.GameUpdate): boolean when the keys apply; default always
---@return games.GameAction[]
function M.moves(reach, available)
  return vim.tbl_map(function(move)
    ---@type games.GameAction
    return {
      key = move.key,
      label = MOVE_LABEL,
      available = available,
      local_fn = function(view, update)
        local at = M.of(view)
        return with_cursor(
          view,
          clamped(at.row + move.d_row, at.col + move.d_col, at.orientation, reach, update)
        )
      end,
    }
  end, keys.HJKL)
end

---Key r turning the cursor between across and down; with a reach it then moves back within
---the bounds of its new way.
---@param available? fun(update: games.GameUpdate): boolean when the key applies; default always
---@param reach? games.CursorReach
---@return games.GameAction
function M.rotate(available, reach)
  return {
    key = ROTATE_KEY,
    label = ROTATE_LABEL,
    available = available,
    local_fn = function(view, update)
      local at = M.of(view)
      local turned = TURNED[at.orientation]
      if reach then
        return with_cursor(view, clamped(at.row, at.col, turned, reach, update))
      end
      return with_cursor(view, { row = at.row, col = at.col, orientation = turned })
    end,
  }
end

---The cells a piece length cells long covers from the cursor, rightward lying across or downward
---lying down, leaving out those past the edges of bounds.
---@param cursor games.GridCursor
---@param length integer
---@param bounds games.CursorBounds
---@return games.GridPos[]
function M.footprint(cursor, length, bounds)
  local cells = {}
  local across = cursor.orientation == M.Orientation.ACROSS
  for i = 0, length - 1 do
    local cell = { row = cursor.row + (across and 0 or i), col = cursor.col + (across and i or 0) }
    if cell.row <= bounds.rows and cell.col <= bounds.cols then
      cells[#cells + 1] = cell
    end
  end
  return cells
end

return M
