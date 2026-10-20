local compose = require("games.ui.compose")
local text = require("games.ui.text")
local theme = require("games.ui.theme")

local CLOSE_KEYS = { "q", "<Esc>" }
local NAMESPACE = vim.api.nvim_create_namespace("games.float")

local M = {}

---@type integer|nil
local open_win

---Shows node in a centred, read-only float that q or <Esc> closes, replacing any previous one.
---@param title string
---@param node games.Node
---@return integer win
function M.open(title, node)
  if open_win and vim.api.nvim_win_is_valid(open_win) then
    vim.api.nvim_win_close(open_win, true)
  end
  theme.setup()
  local rendered = compose.render(node)
  local buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, rendered.lines)
  for _, mark in ipairs(rendered.extmarks) do
    vim.api.nvim_buf_set_extmark(buf, NAMESPACE, mark.row, mark.col, {
      end_col = mark.end_col,
      hl_group = mark.hl,
    })
  end
  vim.bo[buf].modifiable = false
  vim.bo[buf].bufhidden = "wipe"

  local width = math.max(text.max_display_width(rendered.lines), vim.fn.strdisplaywidth(title))
  local height = #rendered.lines
  local win = vim.api.nvim_open_win(buf, true, {
    relative = "editor",
    width = width,
    height = height,
    row = math.floor((vim.o.lines - height) / 2),
    col = math.floor((vim.o.columns - width) / 2),
    style = "minimal",
    border = "rounded",
    title = title,
    title_pos = "center",
  })
  open_win = win
  for _, key in ipairs(CLOSE_KEYS) do
    vim.keymap.set("n", key, function()
      vim.api.nvim_win_close(win, true)
    end, { buffer = buf, nowait = true, desc = "games: close" })
  end
  return win
end

return M
