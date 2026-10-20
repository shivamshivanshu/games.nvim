---@param line string
---@return games.ArtEntry
local function glyph(line)
  return { h = 1, w = 3, lines = { line } }
end

---@param lines string[] as many lines as the square cell is tall, each twice that wide
---@return games.ArtEntry
local function sprite(lines)
  return { h = #lines, w = 2 * #lines, lines = lines }
end

---@param x string
---@param o string
---@return games.ArtSet the mark's key and each empty cell's number
local function glyph_set(x, o)
  local set = { x = { glyph(x) }, o = { glyph(o) } }
  for cell = 1, 9 do
    set[tostring(cell)] = { glyph((" %d "):format(cell)) }
  end
  return set
end

---Tic-Tac-Toe marks: a cross and a ring, with block art for cells 2 to 6 lines tall.
---@type games.AssetPack
return {
  name = "default",
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
}
