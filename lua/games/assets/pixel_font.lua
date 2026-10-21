local text = require("games.ui.text")

---Pixel shapes for block-art packs, "#" lit, and their drawing in block characters.
local M = {}

---3x5 pixel digits.
---@type table<string, string[]>
M.DIGITS = {
  ["0"] = { "###", "# #", "# #", "# #", "###" },
  ["1"] = { " # ", "## ", " # ", " # ", "###" },
  ["2"] = { "## ", "  #", " # ", "#  ", "###" },
  ["3"] = { "## ", "  #", " # ", "  #", "## " },
  ["4"] = { "# #", "# #", "###", "  #", "  #" },
  ["5"] = { "###", "#  ", "## ", "  #", "## " },
  ["6"] = { " ##", "#  ", "###", "# #", "###" },
  ["7"] = { "###", "  #", " # ", " # ", " # " },
  ["8"] = { "###", "# #", "###", "# #", "###" },
  ["9"] = { "###", "# #", "###", "  #", "## " },
}

---5x5 pixel lower-case letters.
---@type table<string, string[]>
M.LETTERS = {
  a = { " ### ", "#   #", "#####", "#   #", "#   #" },
  b = { "#### ", "#   #", "#### ", "#   #", "#### " },
  c = { " ####", "#    ", "#    ", "#    ", " ####" },
  d = { "#### ", "#   #", "#   #", "#   #", "#### " },
  e = { "#####", "#    ", "#### ", "#    ", "#####" },
  f = { "#####", "#    ", "#### ", "#    ", "#    " },
  g = { " ####", "#    ", "#  ##", "#   #", " ####" },
  h = { "#   #", "#   #", "#####", "#   #", "#   #" },
  i = { "#####", "  #  ", "  #  ", "  #  ", "#####" },
  j = { "  ###", "   # ", "   # ", "#  # ", " ##  " },
  k = { "#   #", "#  # ", "###  ", "#  # ", "#   #" },
  l = { "#    ", "#    ", "#    ", "#    ", "#####" },
  m = { "#   #", "## ##", "# # #", "#   #", "#   #" },
  n = { "#   #", "##  #", "# # #", "#  ##", "#   #" },
  o = { " ### ", "#   #", "#   #", "#   #", " ### " },
  p = { "#### ", "#   #", "#### ", "#    ", "#    " },
  q = { " ### ", "#   #", "# # #", "#  # ", " ## #" },
  r = { "#### ", "#   #", "#### ", "#  # ", "#   #" },
  s = { " ####", "#    ", " ### ", "    #", "#### " },
  t = { "#####", "  #  ", "  #  ", "  #  ", "  #  " },
  u = { "#   #", "#   #", "#   #", "#   #", " ### " },
  v = { "#   #", "#   #", "#   #", " # # ", "  #  " },
  w = { "#   #", "#   #", "# # #", "## ##", "#   #" },
  x = { "#   #", " # # ", "  #  ", " # # ", "#   #" },
  y = { "#   #", " # # ", "  #  ", "  #  ", "  #  " },
  z = { "#####", "   # ", "  #  ", " #   ", "#####" },
}

---Each pixel pair stacked in one line: upper, lower, both or neither lit.
local HALF_BLOCKS =
  { [" "] = { [" "] = " ", ["#"] = "▄" }, ["#"] = { [" "] = "▀", ["#"] = "█" } }

---@param pixels string[]
---@param scale integer
---@return string[] each pixel scale by scale
local function scaled(pixels, scale)
  local rows = {}
  for _, row in ipairs(pixels) do
    local wide = row:gsub(".", function(pixel)
      return pixel:rep(scale)
    end)
    for _ = 1, scale do
      rows[#rows + 1] = wide
    end
  end
  return rows
end

---@param pixels string[]
---@param scale? integer each pixel drawn scale by scale; 1 by default
---@return string[] the pixels at one column each, two rows to a line
function M.half_height(pixels, scale)
  local rows = scaled(pixels, scale or 1)
  local lines = {}
  for row = 1, #rows, 2 do
    local upper, lower = rows[row], rows[row + 1] or text.spaces(#rows[row])
    local line = {}
    for col = 1, #upper do
      line[col] = HALF_BLOCKS[upper:sub(col, col)][lower:sub(col, col)]
    end
    lines[#lines + 1] = table.concat(line)
  end
  return lines
end

---@param pixels string[]
---@return string[] the pixels at two full-block columns each, one row to a line
function M.full_size(pixels)
  return vim.tbl_map(function(row)
    return (row:gsub(" ", "  "):gsub("#", "██"))
  end, pixels)
end

---The pixels of digits side by side, one blank column apart.
---@param digits string
---@return string[]
function M.number(digits)
  local rows = {}
  for row = 1, #M.DIGITS["0"] do
    rows[row] = table.concat(
      vim.tbl_map(function(digit)
        return M.DIGITS[digit][row]
      end, text.chars(digits)),
      " "
    )
  end
  return rows
end

---@param pixels string[]
---@param width integer at least the pixels' width
---@return string[] the pixels centred in width columns, odd padding on the right
function M.centred(pixels, width)
  local spare = width - #pixels[1]
  local left, right = (" "):rep(math.floor(spare / 2)), (" "):rep(math.ceil(spare / 2))
  return vim.tbl_map(function(row)
    return left .. row .. right
  end, pixels)
end

---@param lines string[] block-character lines of equal display width
---@return games.ArtEntry
function M.entry(lines)
  return { h = #lines, w = vim.fn.strchars(lines[1]), lines = lines }
end

return M
