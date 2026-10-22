local art = require("games.ui.art")
local build = require("games.assets.shared.build")
local font = require("games.assets.pixel_font")
local notation = require("games.ui.notation")
local text = require("games.ui.text")

---How a marked letter draws without colour, beside its plain look.
---@enum games.LetterMarker
local Marker = { HIT = "hit", PRESENT = "present" }

---An unmarked letter's one-line tile.
local PLAIN_TILE = " %s "
---Each marker's one-line tile of an upper-case letter.
---@type table<games.LetterMarker, string>
local TILE_FORMATS = { [Marker.HIT] = "[%s]", [Marker.PRESENT] = "(%s)" }

---@param lines integer
---@return string[] the spare right column of an unmarked letter's block art
local function plain_column(lines)
  return vim.fn["repeat"]({ " " }, lines)
end

---Each marker in the spare right column of the block art, top to bottom.
---@type table<games.LetterMarker, fun(lines: integer): string[]>
local MARKER_COLUMNS = {
  [Marker.HIT] = function(lines)
    return vim.fn["repeat"]({ "▐" }, lines)
  end,
  [Marker.PRESENT] = function(lines)
    local column = plain_column(lines)
    column[lines] = "▗"
    return column
  end,
}

---Block-art scales of the letters: 3 and 6 lines tall.
local SCALES = { 1, 2 }
---A tile's lines at scale 1: the letter's five pixel rows at half height.
local LINES_PER_SCALE = 3

---Letter tiles shared by the word games: a one-line tile, and block letters 3 and 6 lines tall
---with the marker in a spare column on the right, a bar for a hit and a dot when present.
local M = {}

M.Marker = Marker
---@type integer[] the lines of each block letter, smallest first
M.HEIGHTS = vim.tbl_map(function(scale)
  return LINES_PER_SCALE * scale
end, SCALES)

---@param letter string lower case
---@param marker? games.LetterMarker
---@return string
function M.glyph(letter, marker)
  return (marker and TILE_FORMATS[marker] or PLAIN_TILE):format(letter:upper())
end

---@param letter string lower case
---@param scale integer
---@param column fun(lines: integer): string[]
---@return games.ArtEntry
local function sprite(letter, scale, column)
  local body = font.half_height(font.LETTERS[letter], scale)
  local h = LINES_PER_SCALE * scale
  while #body < h do
    body[#body + 1] = text.spaces(vim.fn.strchars(body[1]))
  end
  local pad, marks = text.spaces(scale - 1), column(h)
  local lines = {}
  for i, line in ipairs(body) do
    lines[i] = line .. pad .. marks[i]
  end
  return build.sprite(lines)
end

---@param letter string lower case
---@param marker? games.LetterMarker
---@return games.ArtEntry[] the letter at each scale, smallest first
function M.art(letter, marker)
  local column = marker and MARKER_COLUMNS[marker] or plain_column
  return vim.tbl_map(function(scale)
    return sprite(letter, scale, column)
  end, SCALES)
end

---@generic T
---@param look fun(letter: string, marker?: games.LetterMarker): T
---@param markers table<string, games.LetterMarker> each key marker -> the marker its letters draw with
---@return table<string, T> each letter's plain look, and its look under each key marker, "hit:a" and so on
function M.by_letter(look, markers)
  local set = {}
  for _, letter in ipairs(notation.LETTERS) do
    set[letter] = look(letter)
    for key_marker, marker in pairs(markers) do
      set[art.marked_key(letter, key_marker)] = look(letter, marker)
    end
  end
  return set
end

return M
