---@param line string
---@return games.ArtEntry
local function glyph(line)
  return { h = 1, w = 3, lines = { line } }
end

---@param lines table<string, string> key -> its one-line glyph
---@return games.ArtSet
local function glyph_set(lines)
  return vim.iter(lines):fold({}, function(set, key, line)
    set[key] = { glyph(line) }
    return set
  end)
end

---Mastermind pegs: each colour is its own letter, circled in unicode, so a peg never rests on
---colour alone; key pegs are a filled dot for black and a ring for white.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    unicode = glyph_set({
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
    }),
    ascii = glyph_set({
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
    }),
  },
  art = {},
}
