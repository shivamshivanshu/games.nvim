local art = require("games.ui.art")
local compose = require("games.ui.compose")
local text_util = require("games.ui.text")

---@alias games.CardKey string rank then suit letter, as on the wire: "As", "Td", "2c"
---@alias games.UnoCardKey string colour letter then face, as on the wire: "R5", "G+2", "W+4"

local RED_HL = "GamesCardRed"
---@type table<string, true> suit letters drawn red
local RED_SUITS = { d = true, h = true }
---@type table<string, string> Uno colour letter -> the group it is drawn in
local UNO_COLOR_HL =
  { R = RED_HL, Y = "GamesCardYellow", G = "GamesCardGreen", B = "GamesCardBlue" }
local CARD_GAP = " "
local MARK = "^"

---Playing cards as games and their asset packs name and draw them: one key per card, the
---wire's rank and suit letters, and hearts and diamonds in red; Uno cards likewise, by colour
---letter and face, in their colour.
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

M.UNO_COLORS = "RYGB"
M.UNO_FACES = { "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "S", "R", "+2" }
M.UNO_WILDS = { "W", "W+4" }

---The 54 distinct Uno card keys: each colour's faces, then the wilds.
---@return games.UnoCardKey[]
function M.uno_keys()
  local keys = {}
  for color in M.UNO_COLORS:gmatch(".") do
    for _, face in ipairs(M.UNO_FACES) do
      keys[#keys + 1] = color .. face
    end
  end
  return vim.list_extend(keys, M.UNO_WILDS)
end

---The foreground a card is drawn in: red for hearts and diamonds, none for the black suits.
---@param card games.CardKey
---@return string|nil
function M.hl(card)
  return RED_SUITS[card:sub(2, 2)] and RED_HL or nil
end

---The foreground an Uno card or colour letter is drawn in: its colour, none for a wild.
---@param card games.UnoCardKey|string
---@return string|nil
function M.uno_hl(card)
  return UNO_COLOR_HL[card:sub(1, 1)]
end

---Cards as their glyphs in set, a space apart, each in its colour.
---@param set games.ArtSet
---@param cards string[]
---@param ambiwidth games.Ambiwidth
---@param hl? fun(card: string): string|nil the colour of a card, default M.hl
---@return games.TextNode[]
function M.nodes(set, cards, ambiwidth, hl)
  local colour_of = hl or M.hl
  local nodes = {}
  for i, card in ipairs(cards) do
    if i > 1 then
      nodes[#nodes + 1] = compose.text(CARD_GAP)
    end
    nodes[#nodes + 1] = compose.text(art.glyph(set, card, ambiwidth), colour_of(card))
  end
  return nodes
end

---@class games.CardHandProps
---@field set games.ArtSet
---@field cards string[]
---@field marked table<string, true> cards to mark with a ^ beneath; none marked draws no mark lines
---@field glyph_width integer display columns of each card's glyph
---@field width integer display columns the hand may take; it wraps to more lines
---@field ambiwidth games.Ambiwidth
---@field hl? fun(card: string): string|nil the colour of a card, default M.hl

---A hand of cards as their glyphs, wrapped to props.width, each line of cards followed by one
---marking those in props.marked.
---@param props games.CardHandProps
---@return games.Node[]
function M.hand(props)
  local per_line =
    math.max(1, math.floor((props.width + #CARD_GAP) / (props.glyph_width + #CARD_GAP)))
  local marking = next(props.marked) ~= nil
  local lines = {}
  for first = 1, #props.cards, per_line do
    local line = vim.list_slice(props.cards, first, first + per_line - 1)
    lines[#lines + 1] = compose.row(M.nodes(props.set, line, props.ambiwidth, props.hl))
    if marking then
      local marks = vim.tbl_map(function(card)
        return text_util.pad_to_width(props.marked[card] and MARK or "", props.glyph_width)
      end, line)
      lines[#lines + 1] = compose.text((table.concat(marks, CARD_GAP):gsub("%s+$", "")))
    end
  end
  return lines
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
