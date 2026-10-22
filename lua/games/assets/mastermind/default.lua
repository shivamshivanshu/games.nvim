local build = require("games.assets.shared.build")
local mastermind = require("games.games.mastermind")

---Mastermind pegs: each colour is its own letter, circled in unicode, so a peg never rests on
---colour alone; key pegs are a filled dot for black and a ring for white.
---@type games.AssetPack
return build.default_pack({
  glyphs = {
    unicode = build.glyph_set({
      ["peg:r"] = " Ⓡ ",
      ["peg:g"] = " Ⓖ ",
      ["peg:b"] = " Ⓑ ",
      ["peg:y"] = " Ⓨ ",
      ["peg:o"] = " Ⓞ ",
      ["peg:p"] = " Ⓟ ",
      hole = " ◦ ",
      hidden = " ? ",
      black = " ● ",
      white = " ○ ",
      none = " · ",
      gap = "   ",
    }, mastermind.glyph_width),
    ascii = build.glyph_set({
      ["peg:r"] = " R ",
      ["peg:g"] = " G ",
      ["peg:b"] = " B ",
      ["peg:y"] = " Y ",
      ["peg:o"] = " O ",
      ["peg:p"] = " P ",
      hole = " . ",
      hidden = " ? ",
      black = " * ",
      white = " + ",
      none = " . ",
      gap = "   ",
    }, mastermind.glyph_width),
  },
  art = {},
})
