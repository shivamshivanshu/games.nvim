local M = {}

---@param lines string[]
---@return integer
function M.max_display_width(lines)
  local width = 0
  for _, line in ipairs(lines) do
    width = math.max(width, vim.fn.strdisplaywidth(line))
  end
  return width
end

---@param line string
---@param width integer
---@return string
function M.pad_to_width(line, width)
  return line .. (" "):rep(width - vim.fn.strdisplaywidth(line))
end

return M
