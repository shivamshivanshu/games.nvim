local art = require("games.ui.art")
local build = require("games.assets.shared.build")
local font = require("games.assets.pixel_font")
local sudoku = require("games.games.sudoku")

---@type string[] the digits a cell holds
local DIGITS = vim.tbl_map(tostring, vim.fn.range(1, 9))
local PLAIN = "plain"

---How a marker frames a digit, without colour: the one-line glyph, and the columns either side
---of the art, by line.
---@class (private) games.SudokuFrame
---@field glyph fun(digit: string): string
---@field sides fun(line: integer, lines: integer): string, string

---@type table<string, games.SudokuFrame>
local FRAMES = {
  [PLAIN] = {
    glyph = function(digit)
      return (" %s "):format(digit)
    end,
    sides = function()
      return " ", " "
    end,
  },
  conflict = {
    glyph = function(digit)
      return ("[%s]"):format(digit)
    end,
    sides = function()
      return "▐", "▌"
    end,
  },
  revealed = {
    glyph = function(digit)
      return ("(%s)"):format(digit)
    end,
    sides = function(line, lines)
      if line == lines then
        return "▝", "▘"
      end
      return " ", " "
    end,
  },
}

---@param digit string
---@param frame string
---@return string
local function key_of(digit, frame)
  return art.marked_key(digit, frame ~= PLAIN and frame or nil)
end

---@param body string[]
---@param frame games.SudokuFrame
---@return games.ArtEntry
local function framed(body, frame)
  local lines = {}
  for i, line in ipairs(body) do
    local left, right = frame.sides(i, #body)
    lines[i] = left .. line .. right
  end
  return build.sprite(lines)
end

---@param empty string
---@return games.ArtSet each digit plain and framed per marker, and the empty cell
local function glyph_set(empty)
  local lines = { empty = (" %s "):format(empty) }
  for _, digit in ipairs(DIGITS) do
    for name, frame in pairs(FRAMES) do
      lines[key_of(digit, name)] = frame.glyph(digit)
    end
  end
  return build.glyph_set(lines, sudoku.glyph_width)
end

---@return games.ArtSet
local function art_set()
  local set = {}
  for _, digit in ipairs(DIGITS) do
    local pixels = font.DIGITS[digit]
    for name, frame in pairs(FRAMES) do
      set[key_of(digit, name)] =
        { framed(font.half_height(pixels), frame), framed(font.full_size(pixels), frame) }
    end
  end
  return set
end

---Sudoku digits: plain, [5] for a conflict and (5) for a digit the solution revealed, so a
---cell's state never rests on colour alone; block-pixel art for cells 3 lines tall and up, with
---bars either side for a conflict and corner ticks for a revealed digit.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    -- "·" is two cells wide under ambiwidth=double, where the ascii "." stands in for it.
    unicode = glyph_set("·"),
    ascii = glyph_set("."),
  },
  art = art_set(),
}
