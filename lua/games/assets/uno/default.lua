local build = require("games.assets.shared.build")
local cards = require("games.ui.cards")
local text = require("games.ui.text")

local GLYPH_WIDTH = 3
---@type table<string, string> face -> its unicode symbol where it has one
local FACE_SYMBOLS = { S = "⊘", R = "⇄" }

local unicode_lines = { deck = "▒▒▒" }
local ascii_lines = { deck = "###" }
for _, card in ipairs(cards.uno_keys()) do
  local color, face = card:sub(1, 1), card:sub(2)
  unicode_lines[card] = text.pad_to_width(color .. (FACE_SYMBOLS[face] or face), GLYPH_WIDTH)
  ascii_lines[card] = text.pad_to_width(card, GLYPH_WIDTH)
end

---Uno cards as colour letter and face, "R7", "G+2", "W+4", the skip "⊘" and the reverse "⇄";
---in ascii as on the wire, "RS", "RR". The draw pile is a card back.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    unicode = build.glyph_set(unicode_lines, GLYPH_WIDTH),
    ascii = build.glyph_set(ascii_lines, GLYPH_WIDTH),
  },
  art = {},
}
