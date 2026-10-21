local notify = require("games.core.notify")
local picker = require("games.ui.picker")
local sources = require("games.core.sources")
local types = require("games.types")

local PickerName = types.PickerName

---@type games.PickerName[] tried in order when no picker is forced; select always works
local DETECTION_ORDER = { PickerName.SNACKS, PickerName.TELESCOPE, PickerName.FZF }

---@class games.FinderHandlers what an adapter calls with the item the user picked
---@field confirm fun(item: any) runs the source's default action, or asks which to run
---@field choose fun(item: any) asks which of the item's actions to run
---@field choose_key string vim key notation of the key that calls choose

---@class games.FinderAdapter
---@field available fun(): boolean whether its picker plugin is installed
---@field open fun(source: games.Source, handlers: games.FinderHandlers)

local M = {}

---@type games.PickerName[]
M.PICKER_NAMES = vim.list_extend(vim.list_slice(DETECTION_ORDER), { PickerName.SELECT })
M.CHOOSE_KEY = "<M-CR>"

---@param name games.PickerName
---@return games.FinderAdapter
function M.adapter(name)
  return require("games.ui.finder." .. name)
end

---The picker to use: forced when it is installed, else the first installed in detection order.
---@param forced? games.PickerName
---@return games.PickerName
function M.picker_name(forced)
  if forced then
    if M.adapter(forced).available() then
      return forced
    end
    notify.say(
      ("games: picker %q is not installed; using vim.ui.select"):format(forced),
      vim.log.levels.WARN
    )
    return PickerName.SELECT
  end
  return vim.iter(DETECTION_ORDER):find(function(name)
    return M.adapter(name).available()
  end) or PickerName.SELECT
end

---Asks which of item's actions to run, or says there is none.
---@param source games.Source
---@param item any
function M.choose(source, item)
  local actions = sources.applicable(source, item)
  if #actions == 0 then
    notify.say("games: nothing to do with " .. source.format(item))
    return
  end
  picker.pick(actions, {
    prompt = source.format(item) .. ":",
    format_item = function(action)
      return action.name
    end,
  }, function(action)
    action.run(item)
  end)
end

---Runs source's default action on item, or asks which action to run when it has none.
---@param source games.Source
---@param item any
function M.confirm(source, item)
  local default = source.default_action
  local action = default and sources.find_action(source, item, { default })
  if action then
    action.run(item)
  else
    M.choose(source, item)
  end
end

---@param source games.Source
---@return games.FinderHandlers
function M.handlers(source)
  return {
    confirm = function(item)
      M.confirm(source, item)
    end,
    choose = function(item)
      M.choose(source, item)
    end,
    choose_key = M.CHOOSE_KEY,
  }
end

---Lists source in the forced or detected picker.
---@param source games.Source
---@param forced? games.PickerName
function M.open(source, forced)
  if #source.items == 0 then
    notify.say(("games: no %s"):format(source.title:lower()))
    return
  end
  M.adapter(M.picker_name(forced)).open(source, M.handlers(source))
end

return M
