local M = {}

---@enum games.Direction
M.Direction = { LEFT = "left", DOWN = "down", UP = "up", RIGHT = "right" }

---@class games.DirectionKey
---@field key string
---@field arrow string the arrow key that means the same
---@field direction games.Direction
---@field d_row integer
---@field d_col integer

---The vi direction keys, in keyboard order.
---@type games.DirectionKey[]
M.HJKL = {
  { key = "h", arrow = "<Left>", direction = M.Direction.LEFT, d_row = 0, d_col = -1 },
  { key = "j", arrow = "<Down>", direction = M.Direction.DOWN, d_row = 1, d_col = 0 },
  { key = "k", arrow = "<Up>", direction = M.Direction.UP, d_row = -1, d_col = 0 },
  { key = "l", arrow = "<Right>", direction = M.Direction.RIGHT, d_row = 0, d_col = 1 },
}

---Maps key in buf's normal mode to fn, described as label.
---@param buf integer
---@param key string
---@param label string
---@param fn fun()
function M.map(buf, key, label, fn)
  vim.keymap.set("n", key, fn, { buffer = buf, nowait = true, desc = "games: " .. label })
end

return M
