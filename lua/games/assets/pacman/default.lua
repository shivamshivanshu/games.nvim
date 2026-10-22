local art = require("games.ui.art")
local build = require("games.assets.shared.build")
local pacman = require("games.games.pacman")

---Cell heights the walls and the door get solid art for, so the maze stays joined in taller cells.
local ART_HEIGHTS = { 2, 3, 4 }

---Each ghost's letter after body, in lower case while frightened, and after a quote as eyes.
---@param body string
---@return table<string, string> every ghost's glyph in each of its looks
local function ghosts(body)
  local lines = {}
  for name, letter in pairs({ blinky = "B", pinky = "P", inky = "I", clyde = "C" }) do
    lines[name] = body .. letter
    lines[name .. "_frightened"] = body .. letter:lower()
    lines[name .. "_eyes"] = '"' .. letter
  end
  return lines
end

---@param tiles table<string, string> the maze tiles' and Pac-Man's glyphs by key
---@param ghost_lines table<string, string>
---@return games.ArtSet
local function glyph_set(tiles, ghost_lines)
  return build.glyph_set(vim.tbl_extend("error", tiles, ghost_lines), pacman.glyph_width)
end

---@param fill string one character
---@return games.ArtEntry[] fill over the whole cell at every height of ART_HEIGHTS
local function solid(fill)
  return vim.tbl_map(function(h)
    return build.sprite(vim.fn["repeat"]({ fill:rep(art.CELL_ASPECT * h) }, h))
  end, ART_HEIGHTS)
end

---Pac-Man's maze: solid walls, a dot, a ring of a pellet, Pac-Man's mouth toward its way, and each
---ghost with its letter, lower case while frightened and behind quotes as eyes, so neither state
---rests on colour alone.
---@type games.AssetPack
return build.default_pack({
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
    wall = solid("█"),
  },
})
