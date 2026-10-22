local M = {}

---Display columns a player's name takes in a game's panel.
M.NAME_WIDTH = 10

---The display columns s takes from the start of a line, as strdisplaywidth counts them.
---@param s string
---@return integer
function M.width(s)
  if s:find("\t", 1, true) then
    return vim.fn.strdisplaywidth(s)
  end
  return vim.api.nvim_strwidth(s)
end

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
    width = math.max(width, M.width(line))
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
  return line .. M.spaces(width - M.width(line))
end

---Centres line in width columns; odd padding goes right.
---@param line string
---@param width integer
---@return string
function M.center_to_width(line, width)
  local padding = width - M.width(line)
  local left = math.floor(padding / 2)
  return M.spaces(left) .. line .. M.spaces(padding - left)
end

---line cut to width characters, then padded to width.
---@param line string
---@param width integer
---@return string
function M.fit_to_width(line, width)
  return M.pad_to_width(vim.fn.strcharpart(line, 0, width), width)
end

---The display width of the widest text_of an item of list.
---@generic T
---@param list T[]
---@param text_of fun(item: T): string
---@return integer
function M.widest(list, text_of)
  return M.max_display_width(vim.tbl_map(text_of, list))
end

---s split into its characters.
---@param s string
---@return string[]
function M.chars(s)
  return vim.fn.split(s, "\\zs")
end

---@param line string
---@param width integer
---@return string
function M.align_right(line, width)
  return M.spaces(width - M.width(line)) .. line
end

return M
