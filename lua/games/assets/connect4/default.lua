---@param line string
---@return games.ArtEntry
local function glyph(line)
  return { h = 1, w = 1, lines = { line } }
end

---Connect4 discs: red solid, yellow hollow, so the seats differ by shape as well as colour.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    unicode = {
      red = { glyph("●") },
      yellow = { glyph("○") },
      -- "·" is two cells wide under ambiwidth=double, where the ascii "." stands in for it.
      empty = { glyph("·") },
    },
    ascii = {
      red = { glyph("X") },
      yellow = { glyph("O") },
      empty = { glyph(".") },
    },
  },
  art = {
    red = {
      { h = 2, w = 4, lines = { "▗██▖", "▝██▘" } },
      { h = 3, w = 6, lines = { " ▄██▄ ", "▐████▌", " ▀██▀ " } },
      {
        h = 4,
        w = 8,
        lines = {
          " ▄████▄ ",
          "▐██████▌",
          "▐██████▌",
          " ▀████▀ ",
        },
      },
      {
        h = 5,
        w = 10,
        lines = {
          "  ▄████▄  ",
          " ████████ ",
          " ████████ ",
          " ████████ ",
          "  ▀████▀  ",
        },
      },
    },
    yellow = {
      { h = 2, w = 4, lines = { "▗▀▀▖", "▝▄▄▘" } },
      { h = 3, w = 6, lines = { " ▄▀▀▄ ", "▐    ▌", " ▀▄▄▀ " } },
      {
        h = 4,
        w = 8,
        lines = { " ▄▀▀▀▀▄ ", "▐      ▌", "▐      ▌", " ▀▄▄▄▄▀ " },
      },
      {
        h = 5,
        w = 10,
        lines = {
          "  ▄▀▀▀▀▄  ",
          " █      █ ",
          " █      █ ",
          " █      █ ",
          "  ▀▄▄▄▄▀  ",
        },
      },
    },
  },
}
