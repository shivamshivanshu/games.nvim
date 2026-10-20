local RANKS = "23456789TJQKA"
---@type table<string, string> wire suit letter -> its symbol
local SUIT_SYMBOLS = { c = "♣", d = "♦", h = "♥", s = "♠" }
local SUITS = "cdhs"
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

local unicode = { empty = { glyph("··") }, blank = { glyph("  ") } }
local ascii = { empty = { glyph("..") }, blank = { glyph("  ") } }
for rank in RANKS:gmatch(".") do
  for suit in SUITS:gmatch(".") do
    unicode[rank .. suit] = { glyph(rank .. SUIT_SYMBOLS[suit]) }
    ascii[rank .. suit] = { glyph(rank .. suit) }
  end
end

local frames = {}
for h = MIN_FRAME_HEIGHT, MAX_FRAME_HEIGHT do
  frames[#frames + 1] = frame(h)
end

---Cards as rank and suit symbol, "A♠", or rank and suit letter, "As", in ascii. Faces draw on
---their cell's card background at any height; an empty board slot is outlined.
---@type games.AssetPack
return {
  name = "default",
  glyphs = { unicode = unicode, ascii = ascii },
  art = { empty = frames },
}
