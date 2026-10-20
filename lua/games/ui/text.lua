local M = {}

---@param n integer blanks; none when n is not positive
---@return string
function M.spaces(n)
  return (" "):rep(n)
end

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
  return line .. M.spaces(width - vim.fn.strdisplaywidth(line))
end

---Centres line in width columns; odd padding goes right.
---@param line string
---@param width integer
---@return string
function M.center_to_width(line, width)
  local padding = width - vim.fn.strdisplaywidth(line)
  local left = math.floor(padding / 2)
  return M.spaces(left) .. line .. M.spaces(padding - left)
end

---@param line string
---@param width integer
---@return string
function M.align_right(line, width)
  return M.spaces(width - vim.fn.strdisplaywidth(line)) .. line
end

return M
