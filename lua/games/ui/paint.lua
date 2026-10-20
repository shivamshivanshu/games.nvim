---@class games.Paint
---@field with_modifiable fun(buf: integer, fn: fun())
---@overload fun(buf: integer, ns: integer, first: integer, last: integer, rendered: games.Rendered)

local M = {}

---Runs fn on a read-only buf made modifiable for it, read-only again even when fn raises.
---@param buf integer
---@param fn fun()
function M.with_modifiable(buf, fn)
  vim.bo[buf].modifiable = true
  local ok, err = pcall(fn)
  vim.bo[buf].modifiable = false
  if not ok then
    error(err, 0)
  end
end

---Replaces lines [first, last) of a read-only buf with rendered, highlighting it in namespace ns.
---@param buf integer
---@param ns integer
---@param first integer
---@param last integer exclusive, -1 for the end of the buffer
---@param rendered games.Rendered
local function paint(buf, ns, first, last, rendered)
  vim.api.nvim_buf_clear_namespace(buf, ns, first, last)
  M.with_modifiable(buf, function()
    vim.api.nvim_buf_set_lines(buf, first, last, false, rendered.lines)
  end)
  for _, mark in ipairs(rendered.extmarks) do
    vim.api.nvim_buf_set_extmark(buf, ns, first + mark.row, mark.col, {
      end_col = mark.end_col,
      hl_group = mark.hl,
    })
  end
end

---@type games.Paint
local painter = setmetatable(M, {
  __call = function(_, ...)
    paint(...)
  end,
})
return painter
