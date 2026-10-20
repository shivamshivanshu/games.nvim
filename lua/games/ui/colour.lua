local M = {}

---@type integer[] xterm's colours 0-15 as 0xRRGGBB
local XTERM_BASE = {
  0x000000,
  0xcd0000,
  0x00cd00,
  0xcdcd00,
  0x0000ee,
  0xcd00cd,
  0x00cdcd,
  0xe5e5e5,
  0x7f7f7f,
  0xff0000,
  0x00ff00,
  0xffff00,
  0x5c5cff,
  0xff00ff,
  0x00ffff,
  0xffffff,
}
local CUBE_LEVELS = { [0] = 0x00, 0x5f, 0x87, 0xaf, 0xd7, 0xff }
local CUBE_START, GREY_START = 16, 232

---@param r integer
---@param g integer
---@param b integer
---@return integer 0xRRGGBB
local function pack(r, g, b)
  return r * 0x10000 + g * 0x100 + b
end

---The colour xterm shows for a 256-colour index.
---@param index integer 0-255
---@return integer 0xRRGGBB
function M.xterm_rgb(index)
  if index < CUBE_START then
    return XTERM_BASE[index + 1]
  end
  if index < GREY_START then
    local cube = index - CUBE_START
    return pack(
      CUBE_LEVELS[math.floor(cube / 36)],
      CUBE_LEVELS[math.floor(cube / 6) % 6],
      CUBE_LEVELS[cube % 6]
    )
  end
  local level = 8 + (index - GREY_START) * 10
  return pack(level, level, level)
end

---@param channel integer 0-255
---@return number
local function linear(channel)
  local c = channel / 255
  return c <= 0.04045 and c / 12.92 or ((c + 0.055) / 1.055) ^ 2.4
end

---WCAG relative luminance, 0 for black to 1 for white.
---@param rgb integer 0xRRGGBB
---@return number
function M.luminance(rgb)
  local r, g, b = math.floor(rgb / 0x10000), math.floor(rgb / 0x100) % 0x100, rgb % 0x100
  return 0.2126 * linear(r) + 0.7152 * linear(g) + 0.0722 * linear(b)
end

---WCAG contrast ratio between two colours, 1 to 21.
---@param a integer 0xRRGGBB
---@param b integer 0xRRGGBB
---@return number
function M.contrast(a, b)
  local la, lb = M.luminance(a), M.luminance(b)
  return (math.max(la, lb) + 0.05) / (math.min(la, lb) + 0.05)
end

return M
