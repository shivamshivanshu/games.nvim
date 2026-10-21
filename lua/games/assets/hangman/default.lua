local wordle = require("games.assets.wordle.default")

local LETTERS = "abcdefghijklmnopqrstuvwxyz"
---Line heights of the block art, matching Wordle's letter sprites.
local ART_HEIGHTS = { 3, 6 }

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
  local rows = vim.tbl_map(function(line)
    return vim.fn.split(line, "\\zs")
  end, figure)
  for i = misses + 1, #PARTS do
    local line, col = unpack(PARTS[i])
    rows[line][col] = " "
  end
  return {
    h = #figure,
    w = #rows[1],
    lines = vim.tbl_map(table.concat, rows),
  }
end

---@param line string
---@return games.ArtEntry
local function glyph(line)
  return { h = 1, w = 3, lines = { line } }
end

---@param mode games.Glyphs
---@return games.ArtSet
local function glyph_set(mode)
  local set = { hidden = { glyph(" _ ") } }
  for letter in LETTERS:gmatch(".") do
    set[letter] = { glyph((" %s "):format(letter:upper())) }
    set["unfound:" .. letter] = { glyph(("(%s)"):format(letter:upper())) }
  end
  for misses = 0, #PARTS do
    set["gallows:" .. misses] = { stage(GALLOWS[mode], misses) }
  end
  return set
end

---A hidden letter in block art: a bar along the bottom of the tile.
---@param h integer
---@return games.ArtEntry
local function hidden_sprite(h)
  local w = 2 * h
  local lines = vim.fn["repeat"]({ (" "):rep(w) }, h - 1)
  lines[h] = ("▀"):rep(w)
  return { h = h, w = w, lines = lines }
end

---Block art: Wordle's letter sprites, its present-marked ones for unfound letters, and a bar for a
---hidden letter.
---@return games.ArtSet
local function art_set()
  local set = { hidden = vim.tbl_map(hidden_sprite, ART_HEIGHTS) }
  for letter in LETTERS:gmatch(".") do
    set[letter] = wordle.art[letter]
    set["unfound:" .. letter] = wordle.art["present:" .. letter]
  end
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
