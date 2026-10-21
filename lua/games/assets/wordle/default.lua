local LETTERS = "abcdefghijklmnopqrstuvwxyz"
local GLYPH_WIDTH = 3

---Each letter as 5 rows of 5 pixels, "#" lit.
---@type table<string, string[]>
local FONT = {
  a = { ".###.", "#...#", "#####", "#...#", "#...#" },
  b = { "####.", "#...#", "####.", "#...#", "####." },
  c = { ".####", "#....", "#....", "#....", ".####" },
  d = { "####.", "#...#", "#...#", "#...#", "####." },
  e = { "#####", "#....", "####.", "#....", "#####" },
  f = { "#####", "#....", "####.", "#....", "#...." },
  g = { ".####", "#....", "#..##", "#...#", ".####" },
  h = { "#...#", "#...#", "#####", "#...#", "#...#" },
  i = { "#####", "..#..", "..#..", "..#..", "#####" },
  j = { "..###", "...#.", "...#.", "#..#.", ".##.." },
  k = { "#...#", "#..#.", "###..", "#..#.", "#...#" },
  l = { "#....", "#....", "#....", "#....", "#####" },
  m = { "#...#", "##.##", "#.#.#", "#...#", "#...#" },
  n = { "#...#", "##..#", "#.#.#", "#..##", "#...#" },
  o = { ".###.", "#...#", "#...#", "#...#", ".###." },
  p = { "####.", "#...#", "####.", "#....", "#...." },
  q = { ".###.", "#...#", "#.#.#", "#..#.", ".##.#" },
  r = { "####.", "#...#", "####.", "#..#.", "#...#" },
  s = { ".####", "#....", ".###.", "....#", "####." },
  t = { "#####", "..#..", "..#..", "..#..", "..#.." },
  u = { "#...#", "#...#", "#...#", "#...#", ".###." },
  v = { "#...#", "#...#", "#...#", ".#.#.", "..#.." },
  w = { "#...#", "#...#", "#.#.#", "##.##", "#...#" },
  x = { "#...#", ".#.#.", "..#..", ".#.#.", "#...#" },
  y = { "#...#", ".#.#.", "..#..", "..#..", "..#.." },
  z = { "#####", "...#.", "..#..", ".#...", "#####" },
}

---Half-block characters by {upper lit, lower lit}.
local HALF_BLOCKS =
  { [false] = { [false] = " ", [true] = "▄" }, [true] = { [false] = "▀", [true] = "█" } }

---Feedback drawn without colour in the spare right column, top to bottom: a bar for a hit, a dot
---for present.
---@type table<string, fun(lines: integer): string[]>
local MARKERS = {
  plain = function(lines)
    return vim.fn["repeat"]({ " " }, lines)
  end,
  hit = function(lines)
    return vim.fn["repeat"]({ "▐" }, lines)
  end,
  present = function(lines)
    local column = vim.fn["repeat"]({ " " }, lines)
    column[lines] = "▗"
    return column
  end,
}

---@param pixels string[] rows of "#" and "."
---@param scale integer each pixel drawn scale by scale
---@return string[] the pixels in half blocks, two pixel rows per line
local function half_blocks(pixels, scale)
  local rows = {}
  for _, row in ipairs(pixels) do
    local scaled = row:gsub(".", function(pixel)
      return pixel:rep(scale)
    end)
    for _ = 1, scale do
      rows[#rows + 1] = scaled
    end
  end
  if #rows % 2 == 1 then
    rows[#rows + 1] = ("."):rep(#rows[1])
  end
  local lines = {}
  for i = 1, #rows, 2 do
    local line = {}
    for col = 1, #rows[i] do
      local upper, lower = rows[i]:sub(col, col) == "#", rows[i + 1]:sub(col, col) == "#"
      line[#line + 1] = HALF_BLOCKS[upper][lower]
    end
    lines[#lines + 1] = table.concat(line)
  end
  return lines
end

---A letter's art at one scale, its marker in a spare column on the right; a blank line pads
---the scaled letter to an even width-to-height cell.
---@param letter string
---@param scale integer
---@param marker fun(lines: integer): string[]
---@return games.ArtEntry
local function sprite(letter, scale, marker)
  local body = half_blocks(FONT[letter], scale)
  local h = 3 * scale
  local pad = (" "):rep(scale - 1)
  while #body < h do
    body[#body + 1] = (" "):rep(5 * scale)
  end
  local column = marker(h)
  local lines = {}
  for i, line in ipairs(body) do
    lines[i] = line .. pad .. column[i]
  end
  return { h = h, w = 2 * h, lines = lines }
end

---The game's key for letter under feedback: the letter itself when plain, else "hit:a" and so on.
---@param feedback string
---@param letter string
---@return string
local function key_of(feedback, letter)
  return feedback == "plain" and letter or ("%s:%s"):format(feedback, letter)
end

---@param line string
---@return games.ArtEntry
local function glyph(line)
  return { h = 1, w = GLYPH_WIDTH, lines = { line } }
end

---How each feedback draws an uppercase letter on a one-line tile.
---@type table<string, fun(upper: string): string>
local TILE_STYLE = {
  plain = function(upper)
    return (" %s "):format(upper)
  end,
  hit = function(upper)
    return ("[%s]"):format(upper)
  end,
  present = function(upper)
    return ("(%s)"):format(upper)
  end,
}

---@param empty string
---@return games.ArtSet
local function glyph_set(empty)
  local set = { empty = { glyph(empty) } }
  for letter in LETTERS:gmatch(".") do
    for feedback, draw in pairs(TILE_STYLE) do
      set[key_of(feedback, letter)] = { glyph(draw(letter:upper())) }
    end
  end
  return set
end

---@return games.ArtSet
local function art_set()
  local set = {}
  for letter in LETTERS:gmatch(".") do
    for feedback, marker in pairs(MARKERS) do
      set[key_of(feedback, letter)] = { sprite(letter, 1, marker), sprite(letter, 2, marker) }
    end
  end
  return set
end

---Wordle tiles: the letter, framed [A] for a hit and (A) when present so feedback never rests on
---colour alone; block letters 3 and 6 lines tall, with a bar or a dot on the right for the same.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    unicode = glyph_set(" · "),
    ascii = glyph_set(" . "),
  },
  art = art_set(),
}
