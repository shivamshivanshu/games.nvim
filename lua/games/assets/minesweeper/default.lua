local font = require("games.assets.pixel_font")

local GLYPH_WIDTH = 2
local MAX_CELL_HEIGHT = 6
local COUNTS = { "1", "2", "3", "4", "5", "6", "7", "8" }

---3x5 pixel icons for the cells that are not a count.
---@type table<string, string[]>
local ICONS = {
  flag = { "## ", "###", "#  ", "#  ", "#  " },
  mine = { " # ", "###", "###", "###", " # " },
  flagged_mine = { "## ", "###", "# #", " ##", " # " },
  exploded = { "# #", " # ", "###", " # ", "# #" },
  wrong_flag = { "# #", "# #", " # ", "# #", "# #" },
}

---@param glyphs table<string, string>
---@return games.ArtSet
local function glyph_set(glyphs)
  local set = {}
  for key, glyph in pairs(glyphs) do
    set[key] = { { h = 1, w = GLYPH_WIDTH, lines = { glyph } } }
  end
  return set
end

---@param pixels string[]
---@return games.ArtEntry[] the pixels at half height, then at full size
local function pixel_art(pixels)
  return { font.entry(font.half_height(pixels)), font.entry(font.full_size(pixels)) }
end

---@return games.ArtSet hidden cells shaded short of their right column and bottom line, so
---touching cells stay apart
local function art_set()
  local hidden = {}
  for h = 2, MAX_CELL_HEIGHT do
    local w = GLYPH_WIDTH * h
    local lines = vim.fn["repeat"]({ ("░"):rep(w - 1) .. " " }, h - 1)
    hidden[#hidden + 1] = font.entry(vim.list_extend(lines, { (" "):rep(w) }))
  end
  local set = { hidden = hidden }
  for _, count in ipairs(COUNTS) do
    set[count] = pixel_art(font.DIGITS[count])
  end
  for key, pixels in pairs(ICONS) do
    set[key] = pixel_art(pixels)
  end
  return set
end

---@param hidden string
---@param flag string
---@param mine string
---@param flagged_mine string
---@param exploded string
---@param wrong_flag string
---@return table<string, string> each key's glyph, counts included
local function glyphs(hidden, flag, mine, flagged_mine, exploded, wrong_flag)
  local by_key = {
    hidden = hidden,
    flag = flag,
    mine = mine,
    flagged_mine = flagged_mine,
    exploded = exploded,
    wrong_flag = wrong_flag,
    ["0"] = "  ",
  }
  for _, count in ipairs(COUNTS) do
    by_key[count] = count .. " "
  end
  return by_key
end

---Minesweeper cells: a shaded hidden cell, the count, a flag, and once the game is over a mine,
---a flagged mine, the exploded mine and a crossed-out wrong flag, each its own glyph so no state rests on
---colour; block art for the counts and icons in cells 3 lines tall and up.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    unicode = glyph_set(glyphs("░░", "⚑ ", "✱ ", "⚑✱", "✸ ", "✗ ")),
    ascii = glyph_set(glyphs("##", "F ", "* ", "F*", "*!", "Fx")),
  },
  art = art_set(),
}
