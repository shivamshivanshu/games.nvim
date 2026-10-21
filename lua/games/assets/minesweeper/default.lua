local build = require("games.assets.shared.build")
local font = require("games.assets.pixel_font")
local minesweeper = require("games.games.minesweeper")

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

---@return games.ArtSet hidden cells shaded short of their right column and bottom line, so
---touching cells stay apart, and the counts' and icons' pixels
local function art_set()
  local set = { hidden = font.inset_block("░", MAX_CELL_HEIGHT) }
  for _, count in ipairs(COUNTS) do
    set[count] = font.icon(font.DIGITS[count])
  end
  for key, pixels in pairs(ICONS) do
    set[key] = font.icon(pixels)
  end
  return set
end

---@param hidden string
---@param flag string
---@param mine string
---@param flagged_mine string
---@param exploded string
---@param wrong_flag string
---@return games.ArtSet each key's glyph, counts included
local function glyph_set(hidden, flag, mine, flagged_mine, exploded, wrong_flag)
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
  return build.glyph_set(by_key, minesweeper.glyph_width)
end

---Minesweeper cells: a shaded hidden cell, the count, a flag, and once the game is over a mine,
---a flagged mine, the exploded mine and a crossed-out wrong flag, each its own glyph so no state rests on
---colour; block art for the counts and icons in cells 3 lines tall and up.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    unicode = glyph_set("░░", "⚑ ", "✱ ", "⚑✱", "✸ ", "✗ "),
    ascii = glyph_set("##", "F ", "* ", "F*", "*!", "Fx"),
  },
  art = art_set(),
}
