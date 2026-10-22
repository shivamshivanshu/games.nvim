local build = require("games.assets.shared.build")
local font = require("games.assets.pixel_font")
local g2048 = require("games.games.g2048")
local text = require("games.ui.text")

local GLYPH_WIDTH = g2048.glyph_width
---Tiles with up to this many digits get block-digit art, all at the width of the widest of them.
local ART_DIGITS = 4
local ART_WIDTH = #font.number(("8"):rep(ART_DIGITS))[1]

---@param empty string the empty cell's mark
---@return games.ArtSet each tile's value, centred
local function glyph_set(empty)
  local lines = { [g2048.EMPTY_KEY] = text.center_to_chars(empty, GLYPH_WIDTH) }
  for _, value in ipairs(g2048.tile_keys) do
    lines[value] = text.center_to_chars(value, GLYPH_WIDTH)
  end
  return build.glyph_set(lines, GLYPH_WIDTH)
end

---@return games.ArtSet the tiles of up to ART_DIGITS digits in block digits
local function art_set()
  local set = {}
  for _, value in ipairs(g2048.tile_keys) do
    if #value <= ART_DIGITS then
      set[value] = { font.number_art(value, ART_WIDTH) }
    end
  end
  return set
end

---2048 tiles: the value, centred, and block digits in cells wide enough for four of them.
---@type games.AssetPack
return build.default_pack({
  glyphs = { unicode = glyph_set("·"), ascii = glyph_set(".") },
  art = art_set(),
})
