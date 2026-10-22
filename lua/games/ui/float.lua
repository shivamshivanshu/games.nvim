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

---Shows node in a centred, read-only float that q or <Esc> closes, replacing any previous one.
---@param title string
---@param node games.Node
---@return integer win
function M.open(title, node)
  if open_win and vim.api.nvim_win_is_valid(open_win) then
    vim.api.nvim_win_close(open_win, true)
  end
  local rendered = compose.render(node)
  local buf = vim.api.nvim_create_buf(false, true)
  paint(buf, NAMESPACE, 0, -1, rendered)
  vim.bo[buf].bufhidden = "wipe"

  local win = vim.api.nvim_open_win(
    buf,
    true,
    vim.tbl_extend("force", M.centred(rendered.lines, text.width(title)), {
      style = "minimal",
      border = "rounded",
      title = title,
      title_pos = "center",
    })
  )
  open_win = win
  for _, key in ipairs(CLOSE_KEYS) do
    keys.map(buf, key, "close", function()
      vim.api.nvim_win_close(win, true)
    end)
  end
  return win
end

return M
