local build = require("games.assets.shared.build")
local ludo = require("games.games.ludo")

---@class (private) games.LudoTokenLook
---@field unicode string
---@field ascii string
---@field art string[][] sprites for a lone token, smallest first

---Each seat's token differs by shape as well as colour.
---@type table<games.LudoSeat, games.LudoTokenLook>
local TOKENS = {
  blue = {
    unicode = "●",
    ascii = "B",
    art = {
      { "▗██▖", "▝██▘" },
      { " ▄▄▄▄ ", "██████", " ▀▀▀▀ " },
    },
  },
  yellow = {
    unicode = "■",
    ascii = "Y",
    art = {
      { "▐██▌", "▐██▌" },
      { "▐████▌", "▐████▌", "▐████▌" },
    },
  },
  red = {
    unicode = "▲",
    ascii = "R",
    art = {
      { " ▟▙ ", "▟██▙" },
      { "  ▟▙  ", " ▟██▙ ", "▟████▙" },
    },
  },
  green = {
    unicode = "◆",
    ascii = "G",
    art = { { " ▟▙ ", " ▜▛ " }, { "  ▟▙  ", " ████ ", "  ▜▛  " } },
  },
}

---A lone token is its shape and a space; a stack adds its count.
---@param shape string
---@param n integer
---@return string
local function stack_glyph(shape, n)
  return shape .. (n == 1 and " " or tostring(n))
end

---@param mode "unicode"|"ascii"
---@param squares table<string, string>
---@param mixed string the shape of several colours on one square, followed by their count
---@return games.ArtSet
local function glyph_set(mode, squares, mixed)
  local lines = vim.deepcopy(squares)
  for _, seat in ipairs(ludo.SEATS) do
    local shape = TOKENS[seat][mode] --[[@as string]]
    for n = 1, ludo.TOKENS_PER_SEAT do
      lines[seat .. n] = stack_glyph(shape, n)
    end
  end
  for n = 2, ludo.MAX_MIXED_COUNT do
    lines["mixed" .. n] = mixed .. n
  end
  return build.glyph_set(lines, ludo.glyph_width)
end

---@return games.ArtSet
local function token_art()
  local set = {}
  for _, seat in ipairs(ludo.SEATS) do
    set[seat .. 1] = vim.tbl_map(build.sprite, TOKENS[seat].art)
  end
  return set
end

---Ludo tokens as a shape per seat, with block art for lone tokens in cells 2 and 3 lines tall;
---several colours on one square show their count, 9 standing for 9 or more.
---@type games.AssetPack
return build.default_pack({
  glyphs = {
    unicode = glyph_set("unicode", {
      track = "· ",
      safe = "✶ ",
      lane = "░░",
      spot = "○ ",
      goal = "⌂ ",
      yard = "  ",
      center = "  ",
      movable = " ←",
    }, "※"),
    ascii = glyph_set("ascii", {
      track = ". ",
      safe = "* ",
      lane = "::",
      spot = "o ",
      goal = "^ ",
      yard = "  ",
      center = "  ",
      movable = " <",
    }, "x"),
  },
  art = token_art(),
})
