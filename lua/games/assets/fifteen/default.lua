local art = require("games.ui.art")
local build = require("games.assets.shared.build")
local fifteen = require("games.games.fifteen")
local font = require("games.assets.pixel_font")
local text = require("games.ui.text")

local MAX_TILE = 15
local MOVABLE_MARKER = "movable"
---Pixel columns of the widest tile, "15": every tile's art takes it, so all scale together.
local ART_WIDTH = #font.number(tostring(MAX_TILE))[1]

---@return games.ArtSet each tile plain and bracketed as movable, and the blank
local function glyph_set()
  local lines = { blank = text.spaces(fifteen.glyph_width) }
  for tile = 1, MAX_TILE do
    lines[tostring(tile)] = (" %2d "):format(tile)
    lines[art.marked_key(tile, MOVABLE_MARKER)] = ("[%2d]"):format(tile)
  end
  return build.glyph_set(lines, fifteen.glyph_width)
end

---@return games.ArtSet each tile in block digits, a movable one underlined
local function art_set()
  local set = {}
  for tile = 1, MAX_TILE do
    set[tostring(tile)] = { font.number_art(tile, ART_WIDTH, nil, text.spaces(ART_WIDTH)) }
    set[art.marked_key(tile, MOVABLE_MARKER)] =
      { font.number_art(tile, ART_WIDTH, nil, ("▀"):rep(ART_WIDTH)) }
  end
  return set
end

local glyphs = glyph_set()

---15-puzzle tiles: the number, bracketed when it may move so that never rests on colour alone,
---and block digits, underlined when movable, in cells 4 lines tall and up.
---@type games.AssetPack
return {
  name = "default",
  glyphs = { unicode = glyphs, ascii = glyphs },
  art = art_set(),
}
