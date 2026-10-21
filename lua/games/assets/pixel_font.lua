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

---Each pixel pair stacked in one line: upper, lower, both or neither lit.
local HALF_BLOCKS =
  { [" "] = { [" "] = " ", ["#"] = "▄" }, ["#"] = { [" "] = "▀", ["#"] = "█" } }

---@param pixels string[]
---@return string[] the pixels at one column each, two rows to a line
function M.half_height(pixels)
  local lines = {}
  for row = 1, #pixels, 2 do
    local upper, lower = pixels[row], pixels[row + 1] or (" "):rep(#pixels[row])
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

---The pixels of text's digits side by side, one blank column apart.
---@param text string digits only
---@return string[]
function M.number(text)
  local rows = {}
  for row = 1, #M.DIGITS["0"] do
    rows[row] = table.concat(
      vim.tbl_map(function(digit)
        return M.DIGITS[digit][row]
      end, vim.split(text, "")),
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
