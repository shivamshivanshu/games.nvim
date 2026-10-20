local art = require("games.ui.art")
local notify = require("games.core.notify")

---@class games.AssetPack
---@field name string
---@field art games.ArtSet multi-line block art, drawn only under glyphs = "unicode"
---@field glyphs table<games.Glyphs, games.ArtSet> one-line glyphs for each glyph mode

local M = {}

M.DEFAULT_PACK = "default"

---@type games.Glyphs[]
local GLYPH_MODES = { "unicode", "ascii" }
local BLOCK_ELEMENT = "\226\150[\128-\159]" -- U+2580-259F in UTF-8

---@type table<string, games.AssetPack> "<game>/<pack>" -> the pack it resolved to
local loaded = {}

---@param value any
---@return boolean
local function is_count(value)
  return type(value) == "number" and value >= 1 and value == math.floor(value)
end

---@param line string
---@return boolean
local function only_blocks_and_spaces(line)
  return line:gsub(BLOCK_ELEMENT, ""):gsub(" ", "") == ""
end

---@param entry any
---@return string|nil problem
local function shape_problem(entry)
  if type(entry) ~= "table" or not is_count(entry.h) or not is_count(entry.w) then
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

---@param entry games.ArtEntry
---@return string|nil problem
local function glyph_problem(entry)
  if entry.h ~= 1 then
    return "a glyph is one line"
  end
end

---@param path string
---@param set games.ArtSet
---@param keys string[]
---@return string|nil problem
local function missing_glyph(path, set, keys)
  for _, key in ipairs(keys) do
    if not vim.iter(set[key] or {}):any(art.drawable) then
      return ("%s.%s: no one-line glyph that draws at its width"):format(path, key)
    end
  end
end

---Why pack cannot draw a game with these keys, or nil when it can.
---@param pack any
---@param keys string[] the game module's asset_keys
---@return string|nil problem
function M.problem(pack, keys)
  if type(pack) ~= "table" or type(pack.glyphs) ~= "table" then
    return "a pack is a table with glyphs"
  end
  local key_set = vim.iter(keys):fold({}, function(acc, key)
    acc[key] = true
    return acc
  end)
  for _, mode in ipairs(GLYPH_MODES) do
    local path, set = "glyphs." .. mode, pack.glyphs[mode]
    local problem = set_problem(path, set, key_set, glyph_problem) or missing_glyph(path, set, keys)
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
---@param keys string[]
---@return games.AssetPack|nil
---@return string|nil problem
local function try_load(game, name, keys)
  local ok, pack = pcall(require, module_name(game, name))
  if not ok then
    return nil, pack
  end
  local problem = M.problem(pack, keys)
  if problem then
    return nil, problem
  end
  return pack
end

---The game's pack called name, validated against keys; a bad pack falls back to the default
---one with a single notice.
---@param game games.GameType
---@param name string a shipped pack or any module name
---@param keys string[] the game module's asset_keys
---@return games.AssetPack
function M.load(game, name, keys)
  local id = game .. "/" .. name
  if loaded[id] then
    return loaded[id]
  end
  local pack, problem = try_load(game, name, keys)
  if not pack then
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
    pack = M.load(game, M.DEFAULT_PACK, keys)
  end
  loaded[id] = pack
  return pack
end

---What a board draws from pack in glyph mode: its glyphs, plus its art under "unicode".
---@param pack games.AssetPack
---@param glyphs games.Glyphs
---@return games.ArtSet
function M.art_set(pack, glyphs)
  local set = pack.glyphs[glyphs]
  if glyphs == "ascii" then
    return set
  end
  return vim.iter(set):fold({}, function(merged, key, entries)
    merged[key] = vim.list_extend(vim.list_extend({}, entries), pack.art[key] or {})
    return merged
  end)
end

return M
