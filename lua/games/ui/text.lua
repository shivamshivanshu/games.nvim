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

---Wire identifiers in words: "draw_agreed" -> "draw agreed".
---@param identifier string
---@return string
function M.humanize(identifier)
  return (identifier:gsub("_", " "))
end

---@param line string
---@param width integer
---@return string
function M.pad_to_width(line, width)
  return line .. (" "):rep(width - vim.fn.strdisplaywidth(line))
end

---@param line string
---@param width integer
---@return string
function M.align_right(line, width)
  return (" "):rep(width - vim.fn.strdisplaywidth(line)) .. line
end

return M
