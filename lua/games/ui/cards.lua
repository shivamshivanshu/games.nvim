local art = require("games.ui.art")
local compose = require("games.ui.compose")

---@alias games.CardKey string rank then suit letter, as on the wire: "As", "Td", "2c"

local RED_HL = "GamesCardRed"
---@type table<string, true> suit letters drawn red
local RED_SUITS = { d = true, h = true }
local CARD_GAP = " "

---Playing cards as games and their asset packs name and draw them: one key per card, the
---wire's rank and suit letters, and hearts and diamonds in red.
local M = {}

M.RANKS = "23456789TJQKA"
M.SUITS = "cdhs"

---The 52 card keys, by rank then suit.
---@return games.CardKey[]
function M.keys()
  local keys = {}
  for rank in M.RANKS:gmatch(".") do
    for suit in M.SUITS:gmatch(".") do
      keys[#keys + 1] = rank .. suit
    end
  end
  return keys
end

---The foreground a card is drawn in: red for hearts and diamonds, none for the black suits.
---@param card games.CardKey
---@return string|nil
function M.hl(card)
  return RED_SUITS[card:sub(2, 2)] and RED_HL or nil
end

---Cards as their glyphs in set, a space apart, each in its colour.
---@param set games.ArtSet
---@param cards games.CardKey[]
---@param ambiwidth games.Ambiwidth
---@return games.TextNode[]
function M.nodes(set, cards, ambiwidth)
  local nodes = {}
  for i, card in ipairs(cards) do
    if i > 1 then
      nodes[#nodes + 1] = compose.text(CARD_GAP)
    end
    nodes[#nodes + 1] = compose.text(art.glyph(set, card, ambiwidth), M.hl(card))
  end
  return nodes
end

---Cards as their glyphs in set, a space apart, uncoloured, for text such as a notification.
---@param set games.ArtSet
---@param cards games.CardKey[]
---@param ambiwidth games.Ambiwidth
---@return string
function M.text(set, cards, ambiwidth)
  return table.concat(
    vim.tbl_map(function(card)
      return art.glyph(set, card, ambiwidth)
    end, cards),
    CARD_GAP
  )
end

return M
