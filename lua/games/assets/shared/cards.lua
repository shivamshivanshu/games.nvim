local art = require("games.ui.art")
local build = require("games.assets.shared.build")
local cards = require("games.ui.cards")

---@type table<string, string> suit letter -> its symbol
local SUIT_SYMBOLS = { c = "♣", d = "♦", h = "♥", s = "♠" }
local GLYPH_WIDTH = 2
local MIN_FRAME_HEIGHT, MAX_FRAME_HEIGHT = 2, 8

---An empty card slot h lines tall, outlined in block quadrants two columns narrower than its
---cell, so that the outlines of neighbouring slots stay apart.
---@param h integer
---@return games.ArtEntry
local function frame(h)
  local inner = art.CELL_ASPECT * h - 4
  local lines = { "▛" .. ("▀"):rep(inner) .. "▜" }
  for i = 2, h - 1 do
    lines[i] = "▌" .. (" "):rep(inner) .. "▐"
  end
  lines[h] = "▙" .. ("▄"):rep(inner) .. "▟"
  return build.sprite(lines)
end

local unicode_lines = { empty = "··", blank = "  ", button = "Ⓓ " }
local ascii_lines = { empty = "..", blank = "  ", button = "D " }
for _, card in ipairs(cards.keys()) do
  local rank, suit = card:sub(1, 1), card:sub(2, 2)
  unicode_lines[card] = rank .. SUIT_SYMBOLS[suit]
  ascii_lines[card] = card
end

---@type table<games.Glyphs, games.ArtSet>
local GLYPHS = {
  unicode = build.glyph_set(unicode_lines, GLYPH_WIDTH),
  ascii = build.glyph_set(ascii_lines, GLYPH_WIDTH),
}
local ART = { empty = vim.tbl_map(frame, vim.fn.range(MIN_FRAME_HEIGHT, MAX_FRAME_HEIGHT)) }

---Playing cards as rank and suit symbol, "A♠", or rank and suit letter, "As", in ascii, shared by
---the card games. Faces draw on their cell's card background at any height; an empty slot is
---outlined. The dealer button is "Ⓓ", or "D" in ascii.
local M = {}

---The pack holding exactly keys, so each game's pack draws its own keys and no others.
---@param keys string[]
---@return games.AssetPack
function M.pack(keys)
  return build.pack(keys, GLYPHS, ART)
end

return M
