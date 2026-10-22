local build = require("games.assets.shared.build")
local tictactoe = require("games.games.tictactoe")

local sprite = build.sprite

---@param x string
---@param o string
---@return games.ArtSet the mark's key and each empty cell's number
local function glyph_set(x, o)
  local lines = { x = x, o = o }
  for cell = 1, 9 do
    lines[tostring(cell)] = (" %d "):format(cell)
  end
  return build.glyph_set(lines, tictactoe.glyph_width)
end

---Tic-Tac-Toe marks: a cross and a ring, with block art for cells 2 to 6 lines tall.
---@type games.AssetPack
return build.default_pack({
  glyphs = {
    -- "○" is two cells wide under ambiwidth=double, where the ascii "O" stands in for it.
    unicode = glyph_set(" ✕ ", " ○ "),
    ascii = glyph_set(" X ", " O "),
  },
  art = {
    x = {
      sprite({ " ▚▞ ", " ▞▚ " }),
      sprite({ "▜▙  ▟▛", " ▝██▘ ", "▟▛  ▜▙" }),
      sprite({
        "▜▙    ▟▛",
        " ▝▜▙▟▛▘ ",
        " ▗▟▛▜▙▖ ",
        "▟▛    ▜▙",
      }),
      sprite({
        "▜█▖    ▗█▛",
        " ▝▜█▖▗█▛▘ ",
        "   ▐██▌   ",
        " ▗▟█▘▝█▙▖ ",
        "▟█▘    ▝█▙",
      }),
      sprite({
        "▜█▙      ▟█▛",
        " ▝▜█▙  ▟█▛▘ ",
        "   ▝▜██▛▘   ",
        "   ▗▟██▙▖   ",
        " ▗▟█▛  ▜█▙▖ ",
        "▟█▛      ▜█▙",
      }),
    },
    o = {
      sprite({ "▞▀▀▚", "▚▄▄▞" }),
      sprite({ "▗▛▀▀▜▖", "▐▌  ▐▌", "▝▙▄▄▟▘" }),
      sprite({
        " ▗▛▀▀▜▖ ",
        "▐▌    ▐▌",
        "▐▌    ▐▌",
        " ▝▙▄▄▟▘ ",
      }),
      sprite({
        " ▗▟▀▀▀▀▙▖ ",
        "▐█▘    ▝█▌",
        "██      ██",
        "▐█▖    ▗█▌",
        " ▝▜▄▄▄▄▛▘ ",
      }),
      sprite({
        "  ▗▟████▙▖  ",
        " ▟█▀    ▀█▙ ",
        "▐█▌      ▐█▌",
        "▐█▌      ▐█▌",
        " ▜█▄    ▄█▛ ",
        "  ▝▜████▛▘  ",
      }),
    },
  },
})
