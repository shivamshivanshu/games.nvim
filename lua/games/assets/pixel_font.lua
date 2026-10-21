local art = require("games.ui.art")
local build = require("games.assets.shared.build")
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

---The shortest cell an inset block is drawn in: at one line a cell shows its glyph.
local MIN_INSET_HEIGHT = 2

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

---value in half-height block digits centred in width pixel columns, odd padding on the right,
---between the lines above and below when given.
---@param value integer|string
---@param width integer at least the digits' pixel width
---@param above? string
---@param below? string
---@return games.ArtEntry
function M.number_art(value, width, above, below)
  local lines = vim.tbl_map(function(row)
    return text.center_to_width(row, width)
  end, M.number(tostring(value)))
  lines = M.half_height(lines)
  if above then
    table.insert(lines, 1, above)
  end
  if below then
    lines[#lines + 1] = below
  end
  return build.sprite(lines)
end

---@param pixels string[]
---@return games.ArtEntry[] the pixels at half height, then at full size
function M.icon(pixels)
  return { build.sprite(M.half_height(pixels)), build.sprite(M.full_size(pixels)) }
end

---A cell's square filled with fill short of its right column and bottom line, so that touching
---cells stay apart, at each cell height up to max_height.
---@param fill string one character
---@param max_height integer
---@return games.ArtEntry[]
function M.inset_block(fill, max_height)
  return vim.tbl_map(function(h)
    local w = art.CELL_ASPECT * h
    local lines = vim.fn["repeat"]({ fill:rep(w - 1) .. " " }, h - 1)
    return build.sprite(vim.list_extend(lines, { text.spaces(w) }))
  end, vim.fn.range(MIN_INSET_HEIGHT, max_height))
end

return M
