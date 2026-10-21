local cards = require("games.ui.cards")

local GLYPH_WIDTH = 3
---@type table<string, string> face -> its unicode symbol where it has one
local FACE_SYMBOLS = { S = "⊘", R = "⇄" }

---@param line string
---@return games.ArtEntry
local function glyph(line)
  return { h = 1, w = GLYPH_WIDTH, lines = { line } }
end

---@param text string
---@return string text padded to the glyph width
local function padded(text)
  return text .. (" "):rep(GLYPH_WIDTH - vim.fn.strdisplaywidth(text))
end

local unicode = { deck = { glyph("▒▒▒") } }
local ascii = { deck = { glyph("###") } }
for _, card in ipairs(cards.uno_keys()) do
  local color, face = card:sub(1, 1), card:sub(2)
  unicode[card] = { glyph(padded(color .. (FACE_SYMBOLS[face] or face))) }
  ascii[card] = { glyph(padded(card)) }
end

---Uno cards as colour letter and face, "R7", "G+2", "W+4", the skip "⊘" and the reverse "⇄";
---in ascii as on the wire, "RS", "RR". The draw pile is a card back.
---@type games.AssetPack
return {
  name = "default",
  glyphs = { unicode = unicode, ascii = ascii },
  art = {},
}
