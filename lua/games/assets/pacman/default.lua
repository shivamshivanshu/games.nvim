local pacman = require("games.games.pacman")

local GLYPH_WIDTH = pacman.glyph_width
---Cell heights the walls and the door get solid art for, so the maze stays joined in taller cells.
local ART_HEIGHTS = { 2, 3, 4 }
---Display columns per cell line above one line tall, as the board scales cells.
local CELL_ASPECT = 2

---@param line string
---@return games.ArtEntry[]
local function glyph(line)
  return { { h = 1, w = GLYPH_WIDTH, lines = { line } } }
end

---Each ghost's letter after body, in lower case while frightened, and after a quote as eyes.
---@param body string
---@return table<string, games.ArtEntry[]> every ghost in each of its looks
local function ghosts(body)
  local set = {}
  for name, letter in pairs({ blinky = "B", pinky = "P", inky = "I", clyde = "C" }) do
    set[name] = glyph(body .. letter)
    set[name .. "_frightened"] = glyph(body .. letter:lower())
    set[name .. "_eyes"] = glyph('"' .. letter)
  end
  return set
end

---@param tiles table<string, string> the maze tiles' and Pac-Man's glyphs by key
---@param ghost_set table<string, games.ArtEntry[]>
---@return games.ArtSet
local function glyph_set(tiles, ghost_set)
  return vim.tbl_extend("error", vim.tbl_map(glyph, tiles), ghost_set)
end

---@param line_of fun(w: integer): string one line of the art, w columns wide
---@return games.ArtEntry[] the art at every height of ART_HEIGHTS
local function solid(line_of)
  return vim.tbl_map(function(h)
    local w = CELL_ASPECT * h
    local lines = {}
    for i = 1, h do
      lines[i] = line_of(w)
    end
    return { h = h, w = w, lines = lines }
  end, ART_HEIGHTS)
end

---Pac-Man's maze: solid walls, a dot, a ring of a pellet, Pac-Man's mouth toward its way, and each
---ghost with its letter, lower case while frightened and behind quotes as eyes, so neither state
---rests on colour alone.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    unicode = glyph_set({
      wall = "██",
      door = "──",
      dot = "· ",
      pellet = "● ",
      empty = "  ",
      pacman_up = "ᗢ ",
      pacman_down = "ᗣ ",
      pacman_left = "ᗤ ",
      pacman_right = "ᗧ ",
    }, ghosts("ᗝ")),
    ascii = glyph_set({
      wall = "##",
      door = "--",
      dot = ". ",
      pellet = "o ",
      empty = "  ",
      pacman_up = "V ",
      pacman_down = "^ ",
      pacman_left = "> ",
      pacman_right = "< ",
    }, ghosts("M")),
  },
  art = {
    wall = solid(function(w)
      return ("█"):rep(w)
    end),
  },
}
