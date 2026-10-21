local GLYPH_WIDTH = 3

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
---@return string[] the pixels at one column each, two rows to a line
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
  return lines
end

---@param pixels string[]
---@return string[] the pixels at two full-block columns each, one row to a line
local function full_size(pixels)
  return vim.tbl_map(function(row)
    return (row:gsub(" ", "  "):gsub("#", "██"))
  end, pixels)
end

---How a marker frames a digit, without colour: the one-line glyph, and the columns either side
---of the art, by line.
---@class (private) games.SudokuFrame
---@field glyph fun(digit: string): string
---@field sides fun(line: integer, lines: integer): string, string

---@type table<string, games.SudokuFrame>
local FRAMES = {
  plain = {
    glyph = function(digit)
      return (" %s "):format(digit)
    end,
    sides = function()
      return " ", " "
    end,
  },
  conflict = {
    glyph = function(digit)
      return ("[%s]"):format(digit)
    end,
    sides = function()
      return "▐", "▌"
    end,
  },
  revealed = {
    glyph = function(digit)
      return ("(%s)"):format(digit)
    end,
    sides = function(line, lines)
      if line == lines then
        return "▝", "▘"
      end
      return " ", " "
    end,
  },
}

---@param digit string
---@param frame string
---@return string
local function key_of(digit, frame)
  return frame == "plain" and digit or ("%s:%s"):format(frame, digit)
end

---@param body string[]
---@param frame games.SudokuFrame
---@return games.ArtEntry
local function framed(body, frame)
  local lines = {}
  for i, line in ipairs(body) do
    local left, right = frame.sides(i, #body)
    lines[i] = left .. line .. right
  end
  return { h = #lines, w = vim.fn.strchars(lines[1]), lines = lines }
end

---@param empty string
---@return games.ArtSet each digit plain and framed per marker, and the empty cell
local function glyph_set(empty)
  local set = { empty = { { h = 1, w = GLYPH_WIDTH, lines = { (" %s "):format(empty) } } } }
  for digit in pairs(FONT) do
    for name, frame in pairs(FRAMES) do
      set[key_of(digit, name)] = { { h = 1, w = GLYPH_WIDTH, lines = { frame.glyph(digit) } } }
    end
  end
  return set
end

---@return games.ArtSet
local function art_set()
  local set = {}
  for digit, pixels in pairs(FONT) do
    for name, frame in pairs(FRAMES) do
      set[key_of(digit, name)] =
        { framed(half_height(pixels), frame), framed(full_size(pixels), frame) }
    end
  end
  return set
end

---Sudoku digits: plain, [5] for a conflict and (5) for a digit the solution revealed, so a
---cell's state never rests on colour alone; block-pixel art for cells 3 lines tall and up, with
---bars either side for a conflict and corner ticks for a revealed digit.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    -- "·" is two cells wide under ambiwidth=double, where the ascii "." stands in for it.
    unicode = glyph_set("·"),
    ascii = glyph_set("."),
  },
  art = art_set(),
}
