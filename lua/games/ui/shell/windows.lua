local ui_windows = require("games.ui.windows")

local M = {}

function M.close_window()
  local tiled = vim.tbl_filter(ui_windows.is_tiled, vim.api.nvim_tabpage_list_wins(0))
  if #tiled > 1 then
    vim.api.nvim_win_close(0, false)
  else
    vim.cmd.enew()
  end
end

---@param buf integer
---@return boolean
function M.is_shown(buf)
  return #vim.fn.win_findbuf(buf) > 0
end

---@param buf integer
---@return integer[]
function M.windows_in_tab(buf)
  local tab = vim.api.nvim_get_current_tabpage()
  return vim.tbl_filter(function(win)
    return vim.api.nvim_win_get_tabpage(win) == tab
  end, vim.fn.win_findbuf(buf))
end

---@param win integer
function M.prepare_window(win)
  local options = vim.wo[win][0]
  options.wrap = false
  options.number = false
  options.relativenumber = false
  options.signcolumn = "no"
  options.foldcolumn = "0"
end

---The text area every one of wins shows in full.
---@param wins integer[]
---@return games.Size
function M.shared_text_area(wins)
  local infos = vim.tbl_map(function(win)
    return vim.fn.getwininfo(win)[1]
  end, wins)
  return {
    w = math.min(unpack(vim.tbl_map(function(info)
      return info.width - info.textoff
    end, infos))),
    h = math.min(unpack(vim.tbl_map(function(info)
      return info.height
    end, infos))),
  }
end

---@param win integer
function M.scroll_to_top(win)
  vim.api.nvim_win_call(win, function()
    vim.fn.winrestview({ topline = 1, lnum = 1, col = 0 })
  end)
end

---Shows buf in the current window, or from a float such as the dashboard in the window it was
---opened over, so a board never takes a float's place.
---@param buf integer
function M.show_tiled(buf)
  local current = vim.api.nvim_get_current_win()
  if not ui_windows.is_tiled(current) then
    local previous = vim.fn.win_getid(vim.fn.winnr("#"))
    local tiled = previous ~= 0 and ui_windows.is_tiled(previous) and previous
      or vim.iter(vim.api.nvim_tabpage_list_wins(0)):find(ui_windows.is_tiled)
    vim.api.nvim_set_current_win(tiled)
  end
  vim.api.nvim_set_current_buf(buf)
end

---@param buf integer
function M.focus(buf)
  local win = vim.iter(vim.fn.win_findbuf(buf)):find(ui_windows.is_tiled)
  if win then
    vim.api.nvim_set_current_win(win)
  else
    M.show_tiled(buf)
  end
end

return M
