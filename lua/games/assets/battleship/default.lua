local font = require("games.assets.pixel_font")

local MAX_CELL_HEIGHT = 6

---3x5 pixel icons for the marks drawn on water and ships.
---@type table<string, string[]>
local ICONS = {
  miss = { "   ", " # ", "# #", " # ", "   " },
  hit = { "# #", "# #", " # ", "# #", "# #" },
  sunk = { "# #", "###", "###", "###", "# #" },
  berth = { "###", "# #", "# #", "# #", "###" },
  blocked = { "###", "  #", " # ", "#  ", "###" },
  aim = { " # ", "   ", "# #", "   ", " # " },
}

---@param line string
---@return games.ArtEntry[]
local function glyph(line)
  return { { h = 1, w = 1, lines = { line } } }
end

---@return games.ArtEntry[] a ship's square filled short of its right column and bottom line at
---each cell height, so touching ships stay apart
local function hull()
  local entries = {}
  for h = 2, MAX_CELL_HEIGHT do
    local w = 2 * h
    local lines = vim.fn["repeat"]({ ("█"):rep(w - 1) .. " " }, h - 1)
    entries[#entries + 1] = font.entry(vim.list_extend(lines, { (" "):rep(w) }))
  end
  return entries
end

---@return games.ArtSet
local function art_set()
  local set = { ship = hull() }
  for key, pixels in pairs(ICONS) do
    set[key] = { font.entry(font.half_height(pixels)), font.entry(font.full_size(pixels)) }
  end
  return set
end

---Battleship squares: water, a ship, a miss, a hit and a sunk ship, the ship being placed where it
---fits, the aim, and either where it may not go; each its own glyph so no state rests on colour,
---with block art in cells 2 lines tall and up.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    unicode = {
      -- "·" and the squares are two cells wide under ambiwidth=double, where the ascii ones stand in.
      empty = glyph("·"),
      ship = glyph("■"),
      miss = glyph("○"),
      hit = glyph("✕"),
      sunk = glyph("⊠"),
      berth = glyph("□"),
      blocked = glyph("▧"),
      aim = glyph("◎"),
    },
    ascii = {
      empty = glyph("."),
      ship = glyph("#"),
      miss = glyph("o"),
      hit = glyph("X"),
      sunk = glyph("*"),
      berth = glyph("+"),
      blocked = glyph("!"),
      aim = glyph("@"),
    },
  },
  art = art_set(),
}
