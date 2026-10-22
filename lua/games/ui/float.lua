local compose = require("games.ui.compose")
local keys = require("games.ui.keys")
local paint = require("games.ui.paint")
local text = require("games.ui.text")

local CLOSE_KEYS = { "q", "<Esc>" }
---Columns and lines kept free around a float, so its border shows.
local EDGE_MARGIN = 2
local NAMESPACE = vim.api.nvim_create_namespace("games.float")

local M = {}

---@type integer|nil
local open_win
---@type integer|nil the buffer open_win was opened for
local open_owner

---@class games.FloatOptions
---@field owner? integer the buffer it is opened for, which is_open asks after
---@field close_key? string closes it too, beside q and <Esc>

---The editor-centred config of a window showing lines, at least min_width wide, cut to the editor
---less EDGE_MARGIN all round.
---@param lines string[]
---@param min_width? integer
---@return vim.api.keyset.win_config
function M.centred(lines, min_width)
  local width = math.min(
    math.max(text.max_display_width(lines), min_width or 0),
    vim.o.columns - 2 * EDGE_MARGIN
  )
  local height = math.max(1, math.min(#lines, vim.o.lines - 2 * EDGE_MARGIN))
  return {
    relative = "editor",
    width = width,
    height = height,
    row = math.floor((vim.o.lines - height) / 2),
    col = math.floor((vim.o.columns - width) / 2),
  }
end

---The config of a centred, bordered float titled title, fitting lines and the title, and at least
---min_width wide.
---@param title string
---@param lines string[]
---@param min_width? integer
---@return vim.api.keyset.win_config
function M.config(title, lines, min_width)
  return vim.tbl_extend("force", M.centred(lines, math.max(text.width(title), min_width or 0)), {
    style = "minimal",
    border = "rounded",
    title = title,
    title_pos = "center",
  })
end

---Closes win unless it is closed already.
---@param win integer
function M.close_if_valid(win)
  if vim.api.nvim_win_is_valid(win) then
    vim.api.nvim_win_close(win, true)
  end
end

---Whether the float open now was opened for owner.
---@param owner integer
---@return boolean
function M.is_open(owner)
  return open_owner == owner and open_win ~= nil and vim.api.nvim_win_is_valid(open_win)
end

---Closes the float open now, if any.
function M.close()
  if open_win then
    M.close_if_valid(open_win)
  end
  open_win, open_owner = nil, nil
end

---Shows node in a centred, read-only float that q or <Esc> closes, replacing any previous one.
---@param title string
---@param node games.Node
---@param opts? games.FloatOptions
---@return integer win
function M.open(title, node, opts)
  M.close()
  local options = opts or {}
  local rendered = compose.render(node)
  local buf = vim.api.nvim_create_buf(false, true)
  paint(buf, NAMESPACE, 0, -1, rendered)
  vim.bo[buf].bufhidden = "wipe"

  local win = vim.api.nvim_open_win(buf, true, M.config(title, rendered.lines))
  open_win, open_owner = win, options.owner
  for _, key in ipairs(vim.list_extend({ options.close_key }, CLOSE_KEYS)) do
    keys.map(buf, key, "close", function()
      vim.api.nvim_win_close(win, true)
    end)
  end
  return win
end

return M
