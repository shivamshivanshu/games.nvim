---A cursor a board's keys move over a grid, lying across or down, for games that place pieces
---longer than a cell. It lives in the board's view options, so moving it only redraws the board.
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

---@class (private) games.CursorMove
---@field key string
---@field d_row integer
---@field d_col integer

---@type games.CursorMove[]
local MOVES = {
  { key = "h", d_row = 0, d_col = -1 },
  { key = "j", d_row = 1, d_col = 0 },
  { key = "k", d_row = -1, d_col = 0 },
  { key = "l", d_row = 0, d_col = 1 },
}
local MOVE_LABEL = "move"
local ROTATE_KEY, ROTATE_LABEL = "r", "rotate"

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

---Keys h, j, k and l moving the cursor one cell, stopping at the edges of bounds.
---@param bounds games.CursorBounds
---@param available? fun(update: games.GameUpdate): boolean when the keys apply; default always
---@return games.GameAction[]
function M.moves(bounds, available)
  return vim.tbl_map(function(move)
    ---@type games.GameAction
    return {
      key = move.key,
      label = MOVE_LABEL,
      available = available,
      local_fn = function(view)
        local at = M.of(view)
        return with_cursor(view, {
          row = math.min(math.max(at.row + move.d_row, 1), bounds.rows),
          col = math.min(math.max(at.col + move.d_col, 1), bounds.cols),
          orientation = at.orientation,
        })
      end,
    }
  end, MOVES)
end

---Key r turning the cursor between across and down.
---@param available? fun(update: games.GameUpdate): boolean when the key applies; default always
---@return games.GameAction
function M.rotate(available)
  return {
    key = ROTATE_KEY,
    label = ROTATE_LABEL,
    available = available,
    local_fn = function(view)
      local at = M.of(view)
      return with_cursor(view, { row = at.row, col = at.col, orientation = TURNED[at.orientation] })
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
