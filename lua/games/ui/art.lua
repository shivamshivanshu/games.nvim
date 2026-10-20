---@class games.ArtEntry
---@field h integer lines
---@field w integer display columns of every line
---@field lines string[]

---@alias games.ArtSet table<string, games.ArtEntry[]> key -> its entries; a glyph is an h = 1 entry

local AMBIWIDTH_DOUBLE = "double"

local M = {}

---@type table<string, table<games.ArtEntry, boolean>> 'ambiwidth' -> entry -> drawable
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

---Whether entry draws at its declared size under the current 'ambiwidth'. Multi-line art is
---never drawn under "double", where U+2580-258F render two cells wide.
---@param entry games.ArtEntry
---@return boolean
function M.drawable(entry)
  local ambiwidth = vim.o.ambiwidth
  local cache = drawable_by_ambiwidth[ambiwidth]
  if cache[entry] == nil then
    cache[entry] = (entry.h == 1 or ambiwidth ~= AMBIWIDTH_DOUBLE)
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

---The first drawable h = 1 entry, else the first h = 1 entry.
---@param entries games.ArtEntry[]
---@return games.ArtEntry
local function glyph_entry(entries)
  local glyphs = vim.tbl_filter(function(entry)
    return entry.h == 1
  end, entries)
  return vim.iter(glyphs):find(M.drawable) or glyphs[1] or entries[1]
end

---@param a games.ArtEntry
---@param b games.ArtEntry
---@return boolean
local function larger(a, b)
  return a.h > b.h or (a.h == b.h and a.w > b.w)
end

---The largest drawable entry of key that fits box, falling back to its h = 1 glyph.
---@param set games.ArtSet
---@param key string
---@param box games.Size
---@return games.ArtEntry
function M.pick(set, key, box)
  local entries = entries_of(set, key)
  local best
  for _, entry in ipairs(entries) do
    local fits = entry.h <= box.h and entry.w <= box.w
    if fits and M.drawable(entry) and (best == nil or larger(entry, best)) then
      best = entry
    end
  end
  return best or glyph_entry(entries)
end

---The one-line glyph of key.
---@param set games.ArtSet
---@param key string
---@return string
function M.glyph(set, key)
  return glyph_entry(entries_of(set, key)).lines[1]
end

return M
