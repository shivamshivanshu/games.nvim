local types = require("games.types")

---@class games.ArtEntry
---@field h integer lines
---@field w integer display columns of every line
---@field lines string[]

---@alias games.ArtSet table<string, games.ArtEntry[]> key -> its entries; a glyph is an h = 1 entry

local M = {}

M.PRINTABLE_ASCII = "^[ -~]*$"
---Display columns per line of a cell's art: terminal cells are about twice as tall as wide.
M.CELL_ASPECT = 2

---@type table<games.Ambiwidth, table<games.ArtEntry, boolean>> ambiwidth -> entry -> drawable
local drawable_by_ambiwidth = vim.defaulttable(function()
  return setmetatable({}, { __mode = "k" })
end)

---@param entry games.ArtEntry
---@return boolean
local function renders_at_declared_size(entry)
  if #entry.lines ~= entry.h then
    return false
  end
  return vim.iter(entry.lines):all(function(line)
    return vim.fn.strdisplaywidth(line) == entry.w
  end)
end

---@param entry games.ArtEntry
---@return boolean
local function is_ascii(entry)
  return vim.iter(entry.lines):all(function(line)
    return line:find(M.PRINTABLE_ASCII) ~= nil
  end)
end

---Whether entry draws at its declared size under ambiwidth, which must be the 'ambiwidth' in
---effect. Multi-line entries other than ascii pictures are never drawn under "double", where
---U+2580-258F render two cells wide.
---@param entry games.ArtEntry
---@param ambiwidth games.Ambiwidth
---@return boolean
function M.drawable(entry, ambiwidth)
  local cache = drawable_by_ambiwidth[ambiwidth]
  if cache[entry] == nil then
    cache[entry] = (entry.h == 1 or ambiwidth ~= types.Ambiwidth.DOUBLE or is_ascii(entry))
      and renders_at_declared_size(entry)
  end
  return cache[entry]
end

---@param set games.ArtSet
---@param key string
---@return games.ArtEntry[]
local function entries_of(set, key)
  local entries = set[key]
  if not entries then
    error(("art: no entry for key %q"):format(key), 3)
  end
  return entries
end

---The first drawable h = 1 entry.
---@param entries games.ArtEntry[]
---@param key string
---@param ambiwidth games.Ambiwidth
---@return games.ArtEntry
local function glyph_entry(entries, key, ambiwidth)
  local glyph = vim.iter(entries):find(function(entry)
    return entry.h == 1 and M.drawable(entry, ambiwidth)
  end)
  if not glyph then
    error(("art: no glyph of key %q draws under ambiwidth=%s"):format(key, ambiwidth), 3)
  end
  return glyph
end

---@param a games.ArtEntry
---@param b games.ArtEntry
---@return boolean
local function larger(a, b)
  return a.h > b.h or (a.h == b.h and a.w > b.w)
end

---The largest drawable entry of key that fits box, falling back to its first drawable glyph.
---@param set games.ArtSet
---@param key string
---@param box games.Size
---@param ambiwidth games.Ambiwidth
---@return games.ArtEntry
function M.pick(set, key, box, ambiwidth)
  local entries = entries_of(set, key)
  local best
  for _, entry in ipairs(entries) do
    local fits = entry.h <= box.h and entry.w <= box.w
    if fits and M.drawable(entry, ambiwidth) and (best == nil or larger(entry, best)) then
      best = entry
    end
  end
  return best or glyph_entry(entries, key, ambiwidth)
end

---The picture of key: its first drawable entry, so a unicode picture that does not draw under
---ambiwidth gives way to its ascii stand-in.
---@param set games.ArtSet
---@param key string
---@param ambiwidth games.Ambiwidth
---@return string[] its lines
function M.picture(set, key, ambiwidth)
  local entries = entries_of(set, key)
  local picture = vim.iter(entries):find(function(entry)
    return M.drawable(entry, ambiwidth)
  end)
  if not picture then
    error(("art: no picture of key %q draws under ambiwidth=%s"):format(key, ambiwidth), 2)
  end
  return picture.lines
end

---The one-line glyph of key.
---@param set games.ArtSet
---@param key string
---@param ambiwidth games.Ambiwidth
---@return string
function M.glyph(set, key, ambiwidth)
  return glyph_entry(entries_of(set, key), key, ambiwidth).lines[1]
end

---The glyphs of keys in a line, gap apart.
---@param set games.ArtSet
---@param keys string[]
---@param gap string
---@param ambiwidth games.Ambiwidth
---@return string
function M.glyph_line(set, keys, gap, ambiwidth)
  return table.concat(
    vim.tbl_map(function(key)
      return M.glyph(set, key, ambiwidth)
    end, keys),
    gap
  )
end

---The asset key of base under marker, such as "conflict:5", or base itself unmarked.
---@param base string|integer
---@param marker? string
---@return string
function M.marked_key(base, marker)
  return marker and ("%s:%s"):format(marker, base) or tostring(base)
end

return M
