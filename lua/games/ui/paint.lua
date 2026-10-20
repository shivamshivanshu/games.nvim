---Replaces lines [first, last) of a read-only buf with rendered, highlighting it in namespace ns.
---@param buf integer
---@param ns integer
---@param first integer
---@param last integer exclusive, -1 for the end of the buffer
---@param rendered games.Rendered
return function(buf, ns, first, last, rendered)
  vim.api.nvim_buf_clear_namespace(buf, ns, first, last)
  vim.bo[buf].modifiable = true
  local ok, err = pcall(vim.api.nvim_buf_set_lines, buf, first, last, false, rendered.lines)
  vim.bo[buf].modifiable = false
  if not ok then
    error(err, 0)
  end
  for _, mark in ipairs(rendered.extmarks) do
    vim.api.nvim_buf_set_extmark(buf, ns, first + mark.row, mark.col, {
      end_col = mark.end_col,
      hl_group = mark.hl,
    })
  end
end
