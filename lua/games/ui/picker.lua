local M = {}

---vim.ui.select that calls on_pick only when the user picked something, and otherwise on_cancel.
---@generic T
---@param items T[]
---@param opts vim.ui.select.Opts
---@param on_pick fun(item: T)
---@param on_cancel? fun()
function M.pick(items, opts, on_pick, on_cancel)
  vim.ui.select(items, opts, function(item)
    if item ~= nil then
      on_pick(item)
    elseif on_cancel then
      on_cancel()
    end
  end)
end

return M
