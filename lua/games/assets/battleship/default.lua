local build = require("games.assets.shared.build")
local font = require("games.assets.pixel_font")

local GLYPH_WIDTH = 1
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

---@return games.ArtSet a ship's square filled short of its right column and bottom line, so
---touching ships stay apart, and each mark's icon
local function art_set()
  local set = { ship = font.inset_block("█", MAX_CELL_HEIGHT) }
  for key, pixels in pairs(ICONS) do
    set[key] = font.icon(pixels)
  end
  return set
end

---Battleship squares: water, a ship, a miss, a hit and a sunk ship, the ship being placed where it
---fits, the aim, and either where it may not go; each its own glyph so no state rests on colour,
---with block art in cells 2 lines tall and up.
---@type games.AssetPack
return build.default_pack({
  glyphs = {
    unicode = build.glyph_set({
      -- "·" and the squares are two cells wide under ambiwidth=double, where the ascii ones stand in.
      empty = "·",
      ship = "■",
      miss = "○",
      hit = "✕",
      sunk = "⊠",
      berth = "□",
      blocked = "▧",
      aim = "◎",
    }, GLYPH_WIDTH),
    ascii = build.glyph_set({
      empty = ".",
      ship = "#",
      miss = "o",
      hit = "X",
      sunk = "*",
      berth = "+",
      blocked = "!",
      aim = "@",
    }, GLYPH_WIDTH),
  },
  art = art_set(),
})
