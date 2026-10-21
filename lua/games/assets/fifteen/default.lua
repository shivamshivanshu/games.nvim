local font = require("games.assets.pixel_font")

local GLYPH_WIDTH = 4
local MAX_TILE = 15
---Pixel columns of the widest tile, "15": every tile's art takes it, so all scale together.
local ART_WIDTH = #font.number(tostring(MAX_TILE))[1]

---@param lines string[]
---@return games.ArtEntry[]
local function glyph(lines)
  return { { h = 1, w = GLYPH_WIDTH, lines = lines } }
end

---@return games.ArtSet each tile plain and bracketed as movable, and the blank
local function glyph_set()
  local set = { blank = glyph({ (" "):rep(GLYPH_WIDTH) }) }
  for tile = 1, MAX_TILE do
    set[tostring(tile)] = glyph({ (" %2d "):format(tile) })
    set["movable:" .. tile] = glyph({ ("[%2d]"):format(tile) })
  end
  return set
end

---@return games.ArtSet each tile in block digits, a movable one underlined
local function art_set()
  local set = {}
  for tile = 1, MAX_TILE do
    local digits = font.half_height(font.centred(font.number(tostring(tile)), ART_WIDTH))
    set[tostring(tile)] =
      { font.entry(vim.list_extend(vim.deepcopy(digits), { (" "):rep(ART_WIDTH) })) }
    set["movable:" .. tile] =
      { font.entry(vim.list_extend(vim.deepcopy(digits), { ("▀"):rep(ART_WIDTH) })) }
  end
  return set
end

---15-puzzle tiles: the number, bracketed when it may move so that never rests on colour alone,
---and block digits, underlined when movable, in cells 4 lines tall and up.
---@type games.AssetPack
return {
  name = "default",
  glyphs = { unicode = glyph_set(), ascii = glyph_set() },
  art = art_set(),
}
