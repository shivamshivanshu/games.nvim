local font = require("games.assets.pixel_font")
local mancala = require("games.games.mancala")

local GLYPH_WIDTH = mancala.glyph_width
local KeyKind, key_for = mancala.KeyKind, mancala.key_for
---Pixel columns of a two-digit count: every count's art takes it, so all scale together.
local ART_WIDTH = #font.number("88")[1]

---@param line string
---@return games.ArtEntry[]
local function glyph(line)
  return { { h = 1, w = GLYPH_WIDTH, lines = { line } } }
end

---@param seeds integer
---@param above string the art's top line
---@param below string the art's bottom line
---@return games.ArtEntry[] the count in block digits between above and below
local function art(seeds, above, below)
  local digits = font.half_height(font.centred(font.number(tostring(seeds)), ART_WIDTH))
  return { font.entry(vim.list_extend(vim.list_extend({ above }, digits), { below })) }
end

---@return games.ArtSet each count as a pit, a legal pit and a store, and the blank corner
local function glyph_set()
  local set = { [mancala.BLANK_KEY] = glyph((" "):rep(GLYPH_WIDTH)) }
  for seeds = 0, mancala.MAX_SEEDS do
    set[key_for(KeyKind.PIT, seeds)] = glyph((" %2d "):format(seeds))
    set[key_for(KeyKind.LEGAL, seeds)] = glyph(("[%2d]"):format(seeds))
    set[key_for(KeyKind.STORE, seeds)] = glyph(("(%2d)"):format(seeds))
  end
  return set
end

---@return games.ArtSet each count in block digits: underlined in a legal pit, between bars in a store
local function art_set()
  local blank, bar_above, bar_below =
    (" "):rep(ART_WIDTH), ("▄"):rep(ART_WIDTH), ("▀"):rep(ART_WIDTH)
  local set = {}
  for seeds = 0, mancala.MAX_SEEDS do
    set[key_for(KeyKind.PIT, seeds)] = art(seeds, blank, blank)
    set[key_for(KeyKind.LEGAL, seeds)] = art(seeds, blank, bar_below)
    set[key_for(KeyKind.STORE, seeds)] = art(seeds, bar_above, bar_below)
  end
  return set
end

---Mancala counts: a pit's seeds plain, bracketed when the viewer may sow it, and in parentheses in
---a store, so neither rests on colour alone; block digits in cells 5 lines tall and up.
---@type games.AssetPack
return {
  name = "default",
  glyphs = { unicode = glyph_set(), ascii = glyph_set() },
  art = art_set(),
}
