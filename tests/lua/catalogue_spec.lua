local helpers = require("helpers")
local registry = require("games.core.registry")
local tables = require("games.core.tables")

local assert_eq = helpers.assert_eq

local REPO_ROOT = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h:h:h")

---@return games.GameType[]
local function module_games()
  local paths = vim.fn.glob(REPO_ROOT .. "/lua/games/games/*.lua", false, true)
  return vim.tbl_map(function(path)
    return vim.fn.fnamemodify(path, ":t:r")
  end, paths)
end

---@class (private) games.test.CataloguedModule
---@field game games.GameType
---@field module games.GameModule
---@field catalogued table<games.ActionKind, games.ActionSpec>

---@return games.test.CataloguedModule[]
local function catalogued_modules()
  return vim.tbl_map(function(info)
    return {
      game = info.id,
      module = assert(registry.game_module(info.id), info.id),
      catalogued = helpers.catalogued(info.id),
    }
  end, helpers.catalog())
end

---Each problem of each catalogued module, found by problems_of.
---@param problems_of fun(game: games.test.CataloguedModule): string[]
---@return string[]
local function problems_of_every(problems_of)
  return vim.iter(catalogued_modules()):map(problems_of):flatten():totable()
end

---@param module games.GameModule
---@return games.GameInput[] the module's typed input and those of its actions
local function typed_inputs(module)
  local of_actions = vim
    .iter(module.actions or {})
    :map(function(entry)
      return entry.input
    end)
    :totable()
  return vim.list_extend({ module.input }, of_actions)
end

---@param module games.GameModule
---@return games.Action[] the actions the module's keys send as declared, not built by act
local function fixed_actions(module)
  return vim
    .iter(module.actions or {})
    :map(function(entry)
      return entry.action
    end)
    :totable()
end

---@param game games.test.CataloguedModule
---@return string[]
local function typed_input_problems(game)
  return vim
    .iter(typed_inputs(game.module))
    :map(function(input)
      local spec = game.catalogued[input.kind]
      if not spec then
        return ("%s: typed %q is not catalogued"):format(game.game, input.kind)
      elseif not spec.input then
        return ("%s: typed %q takes no input field in the catalogue"):format(game.game, input.kind)
      end
    end)
    :totable()
end

---@param game games.test.CataloguedModule
---@return string[]
local function fixed_action_problems(game)
  return vim
    .iter(fixed_actions(game.module))
    :map(function(action)
      local spec = game.catalogued[action.kind]
      if not spec then
        return ("%s: %q is not catalogued"):format(game.game, action.kind)
      end
      local sent = vim.tbl_filter(function(field)
        return field ~= "kind"
      end, tables.sorted_keys(action))
      local taken = spec.input and { spec.input.field } or {}
      if not vim.deep_equal(sent, taken) then
        return ("%s: %q sends %s where the catalogue takes %s"):format(
          game.game,
          action.kind,
          vim.inspect(sent),
          vim.inspect(taken)
        )
      end
      local value = spec.input and action[spec.input.field]
      if value and #spec.input.values > 0 and not vim.tbl_contains(spec.input.values, value) then
        return ("%s: %q sends %s %q, not a catalogued value"):format(
          game.game,
          action.kind,
          spec.input.field,
          value
        )
      end
    end)
    :totable()
end

local T = {}

function T.every_game_module_is_catalogued_and_every_catalogued_game_has_a_module()
  local catalogued = vim.tbl_map(function(info)
    return info.id
  end, helpers.catalog())
  table.sort(catalogued)
  local modules = module_games()
  table.sort(modules)
  assert_eq(catalogued, modules)
end

function T.every_typed_input_is_a_catalogued_action_that_takes_an_input_field()
  assert_eq({}, problems_of_every(typed_input_problems))
end

function T.every_fixed_action_sends_its_catalogued_kind_with_only_its_field_and_a_listed_value()
  assert_eq({}, problems_of_every(fixed_action_problems))
end

return T
