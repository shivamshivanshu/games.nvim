---@param line string
---@return games.ArtEntry
local function glyph(line)
  return { h = 1, w = 1, lines = { line } }
end

---3x5 pixel digits, "#" lit.
---@type table<string, string[]>
local FONT = {
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
---@return games.ArtEntry the pixels at one column each, two rows to a line
local function half_height(pixels)
  local lines = {}
  for row = 1, #pixels, 2 do
    local upper, lower = pixels[row], pixels[row + 1] or (" "):rep(#pixels[row])
    local line = {}
    for col = 1, #upper do
      line[col] = HALF_BLOCKS[upper:sub(col, col)][lower:sub(col, col)]
    end
    lines[#lines + 1] = table.concat(line)
  end
  return { h = #lines, w = #pixels[1], lines = lines }
end

---@param pixels string[]
---@return games.ArtEntry the pixels at two full-block columns each, one row to a line
local function full_size(pixels)
  local lines = vim.tbl_map(function(row)
    return (row:gsub(" ", "  "):gsub("#", "██"))
  end, pixels)
  return { h = #lines, w = 2 * #pixels[1], lines = lines }
end

---@param empty string
---@return games.ArtSet each digit as itself, and the empty cell
local function glyph_set(empty)
  local set = { empty = { glyph(empty) } }
  for digit in pairs(FONT) do
    set[digit] = { glyph(digit) }
  end
  return set
end

---Sudoku digits: the digits themselves, with block-pixel art for cells 3 lines tall and up.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    -- "·" is two cells wide under ambiwidth=double, where the ascii "." stands in for it.
    unicode = glyph_set("·"),
    ascii = glyph_set("."),
  },
  art = vim.iter(FONT):fold({}, function(set, digit, pixels)
    set[digit] = { half_height(pixels), full_size(pixels) }
    return set
  end),
}
