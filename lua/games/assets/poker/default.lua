local cards = require("games.ui.cards")

---@type table<string, string> suit letter -> its symbol
local SUIT_SYMBOLS = { c = "♣", d = "♦", h = "♥", s = "♠" }
local GLYPH_WIDTH = 2
local MIN_FRAME_HEIGHT, MAX_FRAME_HEIGHT = 2, 8

---@param line string
---@return games.ArtEntry
local function glyph(line)
  return { h = 1, w = GLYPH_WIDTH, lines = { line } }
end

---An empty card slot h lines tall, outlined in block quadrants two columns narrower than its
---2h wide cell, so that the outlines of neighbouring slots stay apart.
---@param h integer
---@return games.ArtEntry
local function frame(h)
  local inner = 2 * h - 4
  local lines = { "▛" .. ("▀"):rep(inner) .. "▜" }
  for i = 2, h - 1 do
    lines[i] = "▌" .. (" "):rep(inner) .. "▐"
  end
  lines[h] = "▙" .. ("▄"):rep(inner) .. "▟"
  return { h = h, w = inner + 2, lines = lines }
end

local unicode = { empty = { glyph("··") }, blank = { glyph("  ") }, button = { glyph("Ⓓ ") } }
local ascii = { empty = { glyph("..") }, blank = { glyph("  ") }, button = { glyph("D ") } }
for _, card in ipairs(cards.keys()) do
  local rank, suit = card:sub(1, 1), card:sub(2, 2)
  unicode[card] = { glyph(rank .. SUIT_SYMBOLS[suit]) }
  ascii[card] = { glyph(card) }
end

local frames = {}
for h = MIN_FRAME_HEIGHT, MAX_FRAME_HEIGHT do
  frames[#frames + 1] = frame(h)
end

---Cards as rank and suit symbol, "A♠", or rank and suit letter, "As", in ascii. Faces draw on
---their cell's card background at any height; an empty board slot is outlined. The dealer
---button is "Ⓓ", or "D" in ascii.
---@type games.AssetPack
return {
  name = "default",
  glyphs = { unicode = unicode, ascii = ascii },
  art = { empty = frames },
}
