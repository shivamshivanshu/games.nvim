local build = require("games.assets.shared.build")

local sprite = build.sprite

local GLYPH_WIDTH = 1

---@type table<games.Glyphs, games.ArtSet>
local GLYPHS = {
  unicode = build.glyph_set({
    black = "●",
    white = "○",
    -- "·" is two cells wide under ambiwidth=double, where the ascii "." stands in for it.
    empty = "·",
    legal = "+",
  }, GLYPH_WIDTH),
  ascii = build.glyph_set({ black = "X", white = "O", empty = ".", legal = "+" }, GLYPH_WIDTH),
}

---Black solid and white hollow, so the seats differ by shape as well as colour.
---@type games.ArtSet
local ART = {
  black = {
    sprite({ "▗██▖", "▝██▘" }),
    sprite({ " ▄██▄ ", "▐████▌", " ▀██▀ " }),
    sprite({
      " ▄████▄ ",
      "▐██████▌",
      "▐██████▌",
      " ▀████▀ ",
    }),
    sprite({
      "  ▄████▄  ",
      " ████████ ",
      " ████████ ",
      " ████████ ",
      "  ▀████▀  ",
    }),
  },
  white = {
    sprite({ "▗▀▀▖", "▝▄▄▘" }),
    sprite({ " ▄▀▀▄ ", "▐    ▌", " ▀▄▄▀ " }),
    sprite({ " ▄▀▀▀▀▄ ", "▐      ▌", "▐      ▌", " ▀▄▄▄▄▀ " }),
    sprite({
      "  ▄▀▀▀▀▄  ",
      " █      █ ",
      " █      █ ",
      " █      █ ",
      "  ▀▄▄▄▄▀  ",
    }),
  },
}

---Black and white discs for the boards of games.ui.discs, shared by gomoku and reversi.
local M = {}

---The pack holding exactly keys, so each game's pack draws its own keys and no others.
---@param keys string[]
---@return games.AssetPack
function M.pack(keys)
  return build.pack(keys, GLYPHS, ART)
end

return M
