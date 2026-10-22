local build = require("games.assets.shared.build")
local font = require("games.assets.pixel_font")

local GLYPH_WIDTH = 1

---3x5 pixel icons of each seat's box: p1 solid, p2 hollow, as their glyphs are.
---@type table<string, string[]>
local ICONS = {
  p1 = { " # ", "###", "###", "###", " # " },
  p2 = { " # ", "# #", "# #", "# #", " # " },
}

---@return games.ArtSet
local function art_set()
  return vim.tbl_map(font.icon, ICONS)
end

---Dots and Boxes: dots, edges open, drawn, drawn last and under the cursor, and boxes, each its own
---glyph so no state rests on colour, with block art for owned boxes in cells 3 lines tall and up.
---@type games.AssetPack
return build.default_pack({
  glyphs = {
    unicode = build.glyph_set({
      -- These are two cells wide under ambiwidth=double, where the ascii ones stand in.
      dot = "·",
      open = " ",
      across = "─",
      down = "│",
      last_across = "━",
      last_down = "┃",
      aim_across = "┄",
      aim_down = "┆",
      blocked = "×",
      box = " ",
      p1 = "●",
      p2 = "○",
    }, GLYPH_WIDTH),
    ascii = build.glyph_set({
      dot = "+",
      open = " ",
      across = "-",
      down = "|",
      last_across = "=",
      last_down = "#",
      aim_across = ".",
      aim_down = ":",
      blocked = "x",
      box = " ",
      p1 = "X",
      p2 = "O",
    }, GLYPH_WIDTH),
  },
  art = art_set(),
})
