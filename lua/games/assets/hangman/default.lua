local art = require("games.ui.art")
local build = require("games.assets.shared.build")
local hangman = require("games.games.hangman")
local letters = require("games.assets.shared.letters")
local text = require("games.ui.text")

local GALLOWS_MARKER = "gallows"
---@type table<string, games.LetterMarker> an unfound letter draws as the shared present letter
local UNFOUND_MARKERS = { unfound = letters.Marker.PRESENT }

---The whole gallows, one figure per glyph mode; the misses draw its body parts in PARTS order.
---@type table<games.Glyphs, string[]>
local GALLOWS = {
  unicode = {
    "  ┌───┐  ",
    "  │   │  ",
    "  │   O  ",
    "  │  ╱│╲ ",
    "  │  ╱ ╲ ",
    "──┴───── ",
  },
  ascii = {
    "  +---+  ",
    "  |   |  ",
    "  |   O  ",
    "  |  /|\\ ",
    "  |  / \\ ",
    "--+----- ",
  },
}

---Where each body part sits, {line, column}: head, body, left arm, right arm, left leg, right leg.
local PARTS = { { 3, 7 }, { 4, 7 }, { 4, 6 }, { 4, 8 }, { 5, 6 }, { 5, 8 } }

---@param figure string[]
---@param misses integer
---@return games.ArtEntry the gallows with the first misses parts of the body drawn
local function stage(figure, misses)
  local rows = vim.tbl_map(text.chars, figure)
  for i = misses + 1, #PARTS do
    local line, col = unpack(PARTS[i])
    rows[line][col] = " "
  end
  return build.sprite(vim.tbl_map(table.concat, rows))
end

---@param mode games.Glyphs
---@return games.ArtSet
local function glyph_set(mode)
  local lines = letters.by_letter(letters.glyph, UNFOUND_MARKERS)
  lines.hidden = " _ "
  local set = build.glyph_set(lines, hangman.glyph_width)
  for misses = 0, #PARTS do
    set[art.marked_key(misses, GALLOWS_MARKER)] = { stage(GALLOWS[mode], misses) }
  end
  return set
end

---A hidden letter in block art: a bar along the bottom of a letter's tile.
---@param h integer
---@return games.ArtEntry
local function hidden_sprite(h)
  local w = art.CELL_ASPECT * h
  local lines = vim.fn["repeat"]({ text.spaces(w) }, h - 1)
  lines[h] = ("▀"):rep(w)
  return build.sprite(lines)
end

---Block art: the shared letters, present-marked for unfound ones, and a bar for a hidden letter.
---@return games.ArtSet
local function art_set()
  local set = letters.by_letter(letters.art, UNFOUND_MARKERS)
  set.hidden = vim.tbl_map(hidden_sprite, letters.HEIGHTS)
  return set
end

---Hangman: found letters plain, letters left unfound at the end in parentheses (in block art, with
---a dot at the bottom right), hidden ones a blank; the gallows gains a body part per miss, in box
---drawing or plain ascii.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    unicode = glyph_set("unicode"),
    ascii = glyph_set("ascii"),
  },
  art = art_set(),
}
