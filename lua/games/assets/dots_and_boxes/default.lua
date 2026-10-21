local font = require("games.assets.pixel_font")

---3x5 pixel icons of each seat's box: p1 solid, p2 hollow, as their glyphs are.
---@type table<string, string[]>
local ICONS = {
  p1 = { " # ", "###", "###", "###", " # " },
  p2 = { " # ", "# #", "# #", "# #", " # " },
}

---@param line string
---@return games.ArtEntry[]
local function glyph(line)
  return { { h = 1, w = 1, lines = { line } } }
end

---@return games.ArtSet
local function art_set()
  local set = {}
  for key, pixels in pairs(ICONS) do
    set[key] = { font.entry(font.half_height(pixels)), font.entry(font.full_size(pixels)) }
  end
  return set
end

---Dots and Boxes: dots, edges open, drawn, drawn last and under the cursor, and boxes, each its own
---glyph so no state rests on colour, with block art for owned boxes in cells 3 lines tall and up.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    unicode = {
      -- These are two cells wide under ambiwidth=double, where the ascii ones stand in.
      dot = glyph("·"),
      open = glyph(" "),
      across = glyph("─"),
      down = glyph("│"),
      last_across = glyph("━"),
      last_down = glyph("┃"),
      aim_across = glyph("┄"),
      aim_down = glyph("┆"),
      blocked = glyph("×"),
      box = glyph(" "),
      p1 = glyph("●"),
      p2 = glyph("○"),
    },
    ascii = {
      dot = glyph("+"),
      open = glyph(" "),
      across = glyph("-"),
      down = glyph("|"),
      last_across = glyph("="),
      last_down = glyph("#"),
      aim_across = glyph("."),
      aim_down = glyph(":"),
      blocked = glyph("x"),
      box = glyph(" "),
      p1 = glyph("X"),
      p2 = glyph("O"),
    },
  },
  art = art_set(),
}
