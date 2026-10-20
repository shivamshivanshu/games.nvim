---@param line string
---@return games.ArtEntry
local function glyph(line)
  return { h = 1, w = 1, lines = { line } }
end

---Draughts glyphs, hollow white and solid black, with disc art for cells 3 to 6 lines tall; a
---king's disc carries a ring.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    unicode = {
      b = { glyph("⛂") },
      B = { glyph("⛃") },
      w = { glyph("⛀") },
      W = { glyph("⛁") },
      -- "·" is two cells wide under ambiwidth=double, where the ascii "." stands in for it.
      empty = { glyph("·") },
      light = { glyph(" ") },
    },
    ascii = {
      b = { glyph("b") },
      B = { glyph("B") },
      w = { glyph("w") },
      W = { glyph("W") },
      empty = { glyph(".") },
      light = { glyph(" ") },
    },
  },
  art = {
    b = {
      {
        h = 3,
        w = 6,
        lines = {
          " ▄▟▙▄ ",
          "▐████▌",
          " ▀▜▛▀ ",
        },
      },
      {
        h = 4,
        w = 8,
        lines = {
          " ▗▄▄▄▄▖ ",
          "▗██████▖",
          "▝██████▘",
          " ▝▀▀▀▀▘ ",
        },
      },
      {
        h = 5,
        w = 10,
        lines = {
          "  ▗▄▄▄▄▖  ",
          " ▟██████▙ ",
          "▐████████▌",
          " ▜██████▛ ",
          "  ▝▀▀▀▀▘  ",
        },
      },
      {
        h = 6,
        w = 12,
        lines = {
          "   ▗▄▄▄▄▖   ",
          " ▗████████▖ ",
          " ██████████ ",
          " ██████████ ",
          " ▝████████▘ ",
          "   ▝▀▀▀▀▘   ",
        },
      },
    },
    B = {
      {
        h = 3,
        w = 6,
        lines = {
          " ▄▞▚▄ ",
          "▐▌▐▌▐▌",
          " ▀▚▞▀ ",
        },
      },
      {
        h = 4,
        w = 8,
        lines = {
          " ▗▄▄▄▄▖ ",
          "▗█▘▄▄▝█▖",
          "▝█▖▀▀▗█▘",
          " ▝▀▀▀▀▘ ",
        },
      },
      {
        h = 5,
        w = 10,
        lines = {
          "  ▗▄▄▄▄▖  ",
          " ▟█▀▀▀▀█▙ ",
          "▐█▌ ██ ▐█▌",
          " ▜█▄▄▄▄█▛ ",
          "  ▝▀▀▀▀▘  ",
        },
      },
      {
        h = 6,
        w = 12,
        lines = {
          "   ▗▄▄▄▄▖   ",
          " ▗██▛▀▀▜██▖ ",
          " ██▘▗▟▙▖▝██ ",
          " ██▖▝▜▛▘▗██ ",
          " ▝██▙▄▄▟██▘ ",
          "   ▝▀▀▀▀▘   ",
        },
      },
    },
    w = {
      {
        h = 3,
        w = 6,
        lines = {
          " ▄▝▘▄ ",
          "▐▌  ▐▌",
          " ▀▗▖▀ ",
        },
      },
      {
        h = 4,
        w = 8,
        lines = {
          " ▗▄▄▄▄▖ ",
          "▗▛    ▜▖",
          "▝▙    ▟▘",
          " ▝▀▀▀▀▘ ",
        },
      },
      {
        h = 5,
        w = 10,
        lines = {
          "  ▗▄▄▄▄▖  ",
          " ▟▛    ▜▙ ",
          "▐█      █▌",
          " ▜▙    ▟▛ ",
          "  ▝▀▀▀▀▘  ",
        },
      },
      {
        h = 6,
        w = 12,
        lines = {
          "   ▗▄▄▄▄▖   ",
          " ▗█▛▀  ▀▜█▖ ",
          " █▌      ▐█ ",
          " █▌      ▐█ ",
          " ▝█▙▄  ▄▟█▘ ",
          "   ▝▀▀▀▀▘   ",
        },
      },
    },
    W = {
      {
        h = 3,
        w = 6,
        lines = {
          " ▄▝▘▄ ",
          "▐▌██▐▌",
          " ▀▗▖▀ ",
        },
      },
      {
        h = 4,
        w = 8,
        lines = {
          " ▗▄▄▄▄▖ ",
          "▗▛▗▄▄▖▜▖",
          "▝▙▝▀▀▘▟▘",
          " ▝▀▀▀▀▘ ",
        },
      },
      {
        h = 5,
        w = 10,
        lines = {
          "  ▗▄▄▄▄▖  ",
          " ▟▛ ▄▄ ▜▙ ",
          "▐█ ▐██▌ █▌",
          " ▜▙ ▀▀ ▟▛ ",
          "  ▝▀▀▀▀▘  ",
        },
      },
      {
        h = 6,
        w = 12,
        lines = {
          "   ▗▄▄▄▄▖   ",
          " ▗█▛▀  ▀▜█▖ ",
          " █▌ ▟██▙ ▐█ ",
          " █▌ ▜██▛ ▐█ ",
          " ▝█▙▄  ▄▟█▘ ",
          "   ▝▀▀▀▀▘   ",
        },
      },
    },
  },
}
