local M = {}

---Whether win is part of the tabpage's layout rather than a floating window.
---@param win integer
---@return boolean
function M.is_tiled(win)
  return vim.api.nvim_win_get_config(win).relative == ""
end

return M
