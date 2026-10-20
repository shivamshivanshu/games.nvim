local MODULE_PREFIX = "games.games."
local GAME_TYPE_PATTERN = "^[%w_]+$"

local M = {}

---@type table<games.GameType, games.GameModule|false> false records a game type with no module
local modules = {}

---@param modname string
---@return games.GameModule|nil
local function load(modname)
  local ok, module = pcall(require, modname)
  if ok then
    return module
  end
  if vim.startswith(tostring(module), ("module '%s' not found"):format(modname)) then
    return nil
  end
  error(module, 0)
end

---The plugin's module for a game type, or nil when it has none; a module that fails to load raises.
---@param game games.GameType
---@return games.GameModule|nil
function M.game_module(game)
  local cached = modules[game]
  if cached == nil then
    cached = game:match(GAME_TYPE_PATTERN) and load(MODULE_PREFIX .. game) or false
    modules[game] = cached
  end
  return cached or nil
end

return M
