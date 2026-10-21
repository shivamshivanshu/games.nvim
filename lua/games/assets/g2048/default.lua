local build = require("games.assets.shared.build")
local font = require("games.assets.pixel_font")
local g2048 = require("games.games.g2048")

local GLYPH_WIDTH = g2048.glyph_width
---Tiles with up to this many digits get block-digit art, all at the width of the widest of them.
local ART_DIGITS = 4
local ART_WIDTH = #font.number(("8"):rep(ART_DIGITS))[1]

---@param text string
---@return string text centred in the glyph width, odd padding on the right
local function centred(text)
  local spare = GLYPH_WIDTH - vim.fn.strchars(text)
  local left = math.floor(spare / 2)
  return (" "):rep(left) .. text .. (" "):rep(spare - left)
end

---@param empty string the empty cell's mark
---@return games.ArtSet each tile's value, centred
local function glyph_set(empty)
  local lines = { [g2048.EMPTY_KEY] = centred(empty) }
  for _, value in ipairs(g2048.tile_keys) do
    lines[value] = centred(value)
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
return {
  name = "default",
  glyphs = { unicode = glyph_set("·"), ascii = glyph_set(".") },
  art = art_set(),
}
