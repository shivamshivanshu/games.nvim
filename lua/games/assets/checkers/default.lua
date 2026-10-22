local build = require("games.assets.shared.build")

local sprite = build.sprite

local GLYPH_WIDTH = 1

---Draughts glyphs, hollow white and solid black, with disc art for cells 3 to 6 lines tall; a
---king's disc carries a ring.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    unicode = build.glyph_set({
      b = "⛂",
      B = "⛃",
      w = "⛀",
      W = "⛁",
      -- "·" is two cells wide under ambiwidth=double, where the ascii "." stands in for it.
      empty = "·",
      light = " ",
    }, GLYPH_WIDTH),
    ascii = build.glyph_set({
      b = "b",
      B = "B",
      w = "w",
      W = "W",
      empty = ".",
      light = " ",
    }, GLYPH_WIDTH),
  },
  art = {
    b = {
      sprite({
        " ▄▟▙▄ ",
        "▐████▌",
        " ▀▜▛▀ ",
      }),
      sprite({
        " ▗▄▄▄▄▖ ",
        "▗██████▖",
        "▝██████▘",
        " ▝▀▀▀▀▘ ",
      }),
      sprite({
        "  ▗▄▄▄▄▖  ",
        " ▟██████▙ ",
        "▐████████▌",
        " ▜██████▛ ",
        "  ▝▀▀▀▀▘  ",
      }),
      sprite({
        "   ▗▄▄▄▄▖   ",
        " ▗████████▖ ",
        " ██████████ ",
        " ██████████ ",
        " ▝████████▘ ",
        "   ▝▀▀▀▀▘   ",
      }),
    },
    B = {
      sprite({
        " ▄▞▚▄ ",
        "▐▌▐▌▐▌",
        " ▀▚▞▀ ",
      }),
      sprite({
        " ▗▄▄▄▄▖ ",
        "▗█▘▄▄▝█▖",
        "▝█▖▀▀▗█▘",
        " ▝▀▀▀▀▘ ",
      }),
      sprite({
        "  ▗▄▄▄▄▖  ",
        " ▟█▀▀▀▀█▙ ",
        "▐█▌ ██ ▐█▌",
        " ▜█▄▄▄▄█▛ ",
        "  ▝▀▀▀▀▘  ",
      }),
      sprite({
        "   ▗▄▄▄▄▖   ",
        " ▗██▛▀▀▜██▖ ",
        " ██▘▗▟▙▖▝██ ",
        " ██▖▝▜▛▘▗██ ",
        " ▝██▙▄▄▟██▘ ",
        "   ▝▀▀▀▀▘   ",
      }),
    },
    w = {
      sprite({
        " ▄▝▘▄ ",
        "▐▌  ▐▌",
        " ▀▗▖▀ ",
      }),
      sprite({
        " ▗▄▄▄▄▖ ",
        "▗▛    ▜▖",
        "▝▙    ▟▘",
        " ▝▀▀▀▀▘ ",
      }),
      sprite({
        "  ▗▄▄▄▄▖  ",
        " ▟▛    ▜▙ ",
        "▐█      █▌",
        " ▜▙    ▟▛ ",
        "  ▝▀▀▀▀▘  ",
      }),
      sprite({
        "   ▗▄▄▄▄▖   ",
        " ▗█▛▀  ▀▜█▖ ",
        " █▌      ▐█ ",
        " █▌      ▐█ ",
        " ▝█▙▄  ▄▟█▘ ",
        "   ▝▀▀▀▀▘   ",
      }),
    },
    W = {
      sprite({
        " ▄▝▘▄ ",
        "▐▌██▐▌",
        " ▀▗▖▀ ",
      }),
      sprite({
        " ▗▄▄▄▄▖ ",
        "▗▛▗▄▄▖▜▖",
        "▝▙▝▀▀▘▟▘",
        " ▝▀▀▀▀▘ ",
      }),
      sprite({
        "  ▗▄▄▄▄▖  ",
        " ▟▛ ▄▄ ▜▙ ",
        "▐█ ▐██▌ █▌",
        " ▜▙ ▀▀ ▟▛ ",
        "  ▝▀▀▀▀▘  ",
      }),
      sprite({
        "   ▗▄▄▄▄▖   ",
        " ▗█▛▀  ▀▜█▖ ",
        " █▌ ▟██▙ ▐█ ",
        " █▌ ▜██▛ ▐█ ",
        " ▝█▙▄  ▄▟█▘ ",
        "   ▝▀▀▀▀▘   ",
      }),
    },
  },
}
