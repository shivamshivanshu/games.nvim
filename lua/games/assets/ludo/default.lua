local GLYPH_WIDTH = 2
local MAX_STACK = 4

---@param line string
---@return games.ArtEntry
local function glyph(line)
  return { h = 1, w = GLYPH_WIDTH, lines = { line } }
end

---@param lines string[]
---@return games.ArtEntry
local function sprite(lines)
  return { h = #lines, w = vim.fn.strdisplaywidth(lines[1]), lines = lines }
end

---@class (private) games.LudoTokenLook
---@field seat string
---@field unicode string
---@field ascii string
---@field art string[][] sprites for a lone token, smallest first

---Each seat's token differs by shape as well as colour.
---@type games.LudoTokenLook[]
local TOKENS = {
  {
    seat = "blue",
    unicode = "●",
    ascii = "B",
    art = {
      { "▗██▖", "▝██▘" },
      { " ▄▄▄▄ ", "██████", " ▀▀▀▀ " },
    },
  },
  {
    seat = "yellow",
    unicode = "■",
    ascii = "Y",
    art = {
      { "▐██▌", "▐██▌" },
      { "▐████▌", "▐████▌", "▐████▌" },
    },
  },
  {
    seat = "red",
    unicode = "▲",
    ascii = "R",
    art = {
      { " ▟▙ ", "▟██▙" },
      { "  ▟▙  ", " ▟██▙ ", "▟████▙" },
    },
  },
  {
    seat = "green",
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
---@return games.ArtSet
local function glyph_set(mode, squares)
  local set = {}
  for key, line in pairs(squares) do
    set[key] = { glyph(line) }
  end
  for _, token in ipairs(TOKENS) do
    for n = 1, MAX_STACK do
      set[token.seat .. n] = { glyph(stack_glyph(token[mode], n)) }
    end
  end
  return set
end

---@return games.ArtSet
local function token_art()
  local set = {}
  for _, token in ipairs(TOKENS) do
    set[token.seat .. 1] = vim.tbl_map(sprite, token.art)
  end
  return set
end

---Ludo tokens as a shape per seat, with block art for lone tokens in cells 2 and 3 lines tall.
---@type games.AssetPack
return {
  name = "default",
  glyphs = {
    unicode = glyph_set("unicode", {
      track = "· ",
      safe = "✶ ",
      lane = "░░",
      spot = "○ ",
      goal = "⌂ ",
      yard = "  ",
      center = "  ",
      mixed = "※ ",
    }),
    ascii = glyph_set("ascii", {
      track = ". ",
      safe = "* ",
      lane = "::",
      spot = "o ",
      goal = "^ ",
      yard = "  ",
      center = "  ",
      mixed = "x ",
    }),
  },
  art = token_art(),
}
