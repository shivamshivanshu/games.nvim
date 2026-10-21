local art = require("games.ui.art")
local build = require("games.assets.shared.build")
local font = require("games.assets.pixel_font")
local mancala = require("games.games.mancala")
local text = require("games.ui.text")

local KeyKind = mancala.KeyKind
---Pixel columns of a two-digit count: every count's art takes it, so all scale together.
local ART_WIDTH = #font.number("88")[1]

---@return games.ArtSet each count as a pit, a legal pit and a store, and the blank corner
local function glyph_set()
  local lines = { [mancala.BLANK_KEY] = text.spaces(mancala.glyph_width) }
  for seeds = 0, mancala.MAX_SEEDS do
    lines[art.marked_key(seeds, KeyKind.PIT)] = (" %2d "):format(seeds)
    lines[art.marked_key(seeds, KeyKind.LEGAL)] = ("[%2d]"):format(seeds)
    lines[art.marked_key(seeds, KeyKind.STORE)] = ("(%2d)"):format(seeds)
  end
  return build.glyph_set(lines, mancala.glyph_width)
end

---@return games.ArtSet each count in block digits: underlined in a legal pit, between bars in a store
local function art_set()
  local blank, bar_above, bar_below =
    text.spaces(ART_WIDTH), ("▄"):rep(ART_WIDTH), ("▀"):rep(ART_WIDTH)
  local set = {}
  for seeds = 0, mancala.MAX_SEEDS do
    set[art.marked_key(seeds, KeyKind.PIT)] = { font.number_art(seeds, ART_WIDTH, blank, blank) }
    set[art.marked_key(seeds, KeyKind.LEGAL)] =
      { font.number_art(seeds, ART_WIDTH, blank, bar_below) }
    set[art.marked_key(seeds, KeyKind.STORE)] =
      { font.number_art(seeds, ART_WIDTH, bar_above, bar_below) }
  end
  return set
end

local glyphs = glyph_set()

---Mancala counts: a pit's seeds plain, bracketed when the viewer may sow it, and in parentheses in
---a store, so neither rests on colour alone; block digits in cells 5 lines tall and up.
---@type games.AssetPack
return {
  name = "default",
  glyphs = { unicode = glyphs, ascii = glyphs },
  art = art_set(),
}
