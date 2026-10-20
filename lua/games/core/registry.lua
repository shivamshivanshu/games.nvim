local MODULE_PREFIX = "games.games."
local GAME_TYPE_PATTERN = "^[%w_]+$"

local M = {}

---@class (private) games.LoadedModule
---@field module? games.GameModule
---@field err? any why the module failed to load

---@type table<games.GameType, games.LoadedModule> missing modules are not cached
local loaded = {}

---@param modname string
---@return games.LoadedModule|nil nil when no such module exists
local function load(modname)
  local ok, result = pcall(require, modname)
  if ok then
    return { module = result }
  end
  if vim.startswith(tostring(result), ("module '%s' not found"):format(modname)) then
    return nil
  end
  return { err = result }
end

---The plugin's module for a game type, or nil when it has none; a module that fails to load raises,
---on every lookup, without running the broken file again.
---@param game games.GameType
---@return games.GameModule|nil
function M.game_module(game)
  local entry = loaded[game]
  if entry == nil then
    local found = game:match(GAME_TYPE_PATTERN) and load(MODULE_PREFIX .. game)
    if not found then
      return nil
    end
    loaded[game] = found
    entry = found
  end
  if entry.err ~= nil then
    error(entry.err, 0)
  end
  return entry.module
end

return M
