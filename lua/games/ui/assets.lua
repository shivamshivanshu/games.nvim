local notify = require("games.core.notify")
local predicates = require("games.core.predicates")

---@class games.AssetPack
---@field name string
---@field art games.ArtSet multi-line block art, drawn only under glyphs = "unicode"
---@field glyphs table<games.Glyphs, games.ArtSet> one-line glyphs for each glyph mode; an ascii
---glyph stands in for a unicode one that does not draw under the current 'ambiwidth'

---@class games.AssetSpec what a game needs of its packs
---@field keys string[]
---@field glyph_width integer display columns of every one-line glyph: the game's cell_width at h = 1

local M = {}

M.DEFAULT_PACK = "default"

---@type games.Glyphs[]
local GLYPH_MODES = { "unicode", "ascii" }
local BLOCK_ELEMENT = "\226\150[\128-\159]" -- U+2580-259F in UTF-8
local PRINTABLE_ASCII = "^[ -~]*$"

---@type table<string, games.AssetPack> "<game>/<pack>" -> the pack it resolved to
local loaded = {}

---@param line string
---@return boolean
local function only_blocks_and_spaces(line)
  return line:gsub(BLOCK_ELEMENT, ""):gsub(" ", "") == ""
end

---@param entry any
---@return string|nil problem
local function shape_problem(entry)
  if
    type(entry) ~= "table"
    or not predicates.is_positive_integer(entry.h)
    or not predicates.is_positive_integer(entry.w)
  then
    return "needs integer h and w of at least 1"
  end
  if type(entry.lines) ~= "table" or #entry.lines ~= entry.h then
    return ("needs exactly h = %d lines"):format(entry.h)
  end
  if not vim.iter(entry.lines):all(function(line)
    return type(line) == "string"
  end) then
    return "lines must be strings"
  end
end

---Art is uncoloured block elements: only spaces and U+2580-259F, each line w of them.
---@param entry games.ArtEntry
---@return string|nil problem
local function art_problem(entry)
  for i, line in ipairs(entry.lines) do
    if not only_blocks_and_spaces(line) then
      return ("line %d holds characters other than spaces and U+2580-259F"):format(i)
    end
    if vim.fn.strchars(line) ~= entry.w then
      return ("line %d is not w = %d wide"):format(i, entry.w)
    end
  end
end

---@param path string where set sits in the pack, e.g. "glyphs.ascii"
---@param set any
---@param keys table<string, true>
---@param check_entry fun(entry: games.ArtEntry): string|nil
---@return string|nil problem
local function set_problem(path, set, keys, check_entry)
  if type(set) ~= "table" then
    return path .. ": must be a table"
  end
  for key, entries in pairs(set) do
    if not keys[key] then
      return ("%s.%s: not a key of this game"):format(path, vim.inspect(key))
    end
    if type(entries) ~= "table" then
      return ("%s.%s: must be a list of entries"):format(path, key)
    end
    for i, entry in ipairs(entries) do
      local problem = shape_problem(entry) or check_entry(entry)
      if problem then
        return ("%s.%s[%d]: %s"):format(path, key, i, problem)
      end
    end
  end
end

---@param glyph_width integer
---@return fun(entry: games.ArtEntry): string|nil
local function glyph_check(glyph_width)
  return function(entry)
    if entry.h ~= 1 then
      return "a glyph is one line"
    end
    if entry.w ~= glyph_width then
      return ("a glyph is the cell width, w = %d"):format(glyph_width)
    end
  end
end

---An ascii glyph draws under any 'ambiwidth', so it can stand in for a unicode one.
---@param glyph_width integer
---@return fun(entry: games.ArtEntry): string|nil
local function ascii_check(glyph_width)
  local check_glyph = glyph_check(glyph_width)
  return function(entry)
    local line = entry.lines[1]
    local printable = line:find(PRINTABLE_ASCII) ~= nil and #line == entry.w
    return check_glyph(entry)
      or (not printable and ("%q is not printable ascii"):format(line) or nil)
  end
end

---@param path string
---@param set games.ArtSet
---@param keys string[]
---@return string|nil problem
local function missing_glyph(path, set, keys)
  for _, key in ipairs(keys) do
    if #(set[key] or {}) == 0 then
      return ("%s.%s: no glyph"):format(path, key)
    end
  end
end

---Why pack cannot draw a game with these needs, or nil when it can.
---@param pack any
---@param spec games.AssetSpec
---@return string|nil problem
function M.problem(pack, spec)
  if type(pack) ~= "table" or type(pack.glyphs) ~= "table" then
    return "a pack is a table with glyphs"
  end
  local key_set = vim.iter(spec.keys):fold({}, function(acc, key)
    acc[key] = true
    return acc
  end)
  local checks = { unicode = glyph_check(spec.glyph_width), ascii = ascii_check(spec.glyph_width) }
  for _, mode in ipairs(GLYPH_MODES) do
    local path, set = "glyphs." .. mode, pack.glyphs[mode]
    local problem = set_problem(path, set, key_set, checks[mode])
      or missing_glyph(path, set, spec.keys)
    if problem then
      return problem
    end
  end
  return set_problem("art", pack.art, key_set, art_problem)
end

---A shipped pack's module, else name itself as a module.
---@param game games.GameType
---@param name string
---@return string
local function module_name(game, name)
  local shipped = ("games.assets.%s.%s"):format(game, name)
  local path = ("lua/games/assets/%s/%s.lua"):format(game, name)
  return #vim.api.nvim_get_runtime_file(path, false) > 0 and shipped or name
end

---@param game games.GameType
---@param name string
---@param spec games.AssetSpec
---@return boolean ok
---@return games.AssetPack|string result the pack, else why it is unusable
local function try_load(game, name, spec)
  return pcall(function()
    local pack = require(module_name(game, name))
    local problem = M.problem(pack, spec)
    if problem then
      error(problem, 0)
    end
    return pack
  end)
end

---The game's pack called name, validated against spec; a bad pack falls back to the default one
---with a single notice.
---@param game games.GameType
---@param name string a shipped pack or any module name
---@param spec games.AssetSpec
---@return games.AssetPack
function M.load(game, name, spec)
  local id = game .. "/" .. name
  if loaded[id] then
    return loaded[id]
  end
  local ok, pack = try_load(game, name, spec)
  if not ok then
    local problem = tostring(pack)
    assert(name ~= M.DEFAULT_PACK, ("games: %s default pack: %s"):format(game, problem))
    notify.say(
      ("games: %s asset pack %q is unusable, using %q: %s"):format(
        game,
        name,
        M.DEFAULT_PACK,
        problem
      ),
      vim.log.levels.WARN
    )
    pack = M.load(game, M.DEFAULT_PACK, spec)
  end
  ---@cast pack games.AssetPack
  loaded[id] = pack
  return pack
end

---What a board draws from pack in glyph mode: its glyphs, and under "unicode" its art, then its
---ascii glyphs as the fallback for unicode ones that do not draw.
---@param pack games.AssetPack
---@param glyphs games.Glyphs
---@return games.ArtSet
function M.art_set(pack, glyphs)
  local ascii = pack.glyphs.ascii
  if glyphs == "ascii" then
    return ascii
  end
  return vim.iter(pack.glyphs.unicode):fold({}, function(merged, key, entries)
    local combined = vim.list_extend({}, entries)
    vim.list_extend(combined, pack.art[key] or {})
    merged[key] = vim.list_extend(combined, ascii[key])
    return merged
  end)
end

---What module draws for game under opts: its chosen pack in opts' glyph mode.
---@param game games.GameType
---@param module games.GameModule
---@param opts games.OpenOptions
---@return games.ArtSet|nil nil for a module without assets
function M.module_art(game, module, opts)
  if not module.asset_keys then
    return nil
  end
  local pack_name = (opts.assets or {})[game] or M.DEFAULT_PACK
  local spec = { keys = module.asset_keys, glyph_width = module.glyph_width or 1 }
  return M.art_set(M.load(game, pack_name, spec), opts.glyphs)
end

return M
