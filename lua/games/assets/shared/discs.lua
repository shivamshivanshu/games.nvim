---@param line string
---@return games.ArtEntry
local function glyph(line)
  return { h = 1, w = 1, lines = { line } }
end

---@type table<games.Glyphs, games.ArtSet>
local GLYPHS = {
  unicode = {
    black = { glyph("●") },
    white = { glyph("○") },
    -- "·" is two cells wide under ambiwidth=double, where the ascii "." stands in for it.
    empty = { glyph("·") },
    legal = { glyph("+") },
  },
  ascii = {
    black = { glyph("X") },
    white = { glyph("O") },
    empty = { glyph(".") },
    legal = { glyph("+") },
  },
}

---Black solid and white hollow, so the seats differ by shape as well as colour.
---@type games.ArtSet
local ART = {
  black = {
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
  white = {
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
}

---@param set games.ArtSet
---@param keys string[]
---@return games.ArtSet
local function only(set, keys)
  local out = {}
  for _, key in ipairs(keys) do
    out[key] = set[key]
  end
  return out
end

---Black and white discs for the boards of games.ui.discs, shared by gomoku and reversi.
local M = {}

---The pack holding exactly keys, so each game's pack draws its own keys and no others.
---@param keys string[]
---@return games.AssetPack
function M.pack(keys)
  return {
    name = "default",
    glyphs = { unicode = only(GLYPHS.unicode, keys), ascii = only(GLYPHS.ascii, keys) },
    art = only(ART, keys),
  }
end

return M
