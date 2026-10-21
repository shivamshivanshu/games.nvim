local keys = require("games.ui.keys")

local M = {}

---@class games.PerValueActions
---@field kind string the action's kind
---@field field string the action field that carries the value
---@field values integer[] each bound to its own number key
---@field label? string default kind
---@field legal fun(view: table): integer[] the values open to the player now

---One action per value, keyed and sent as the value's digits, available while legal lists it.
---@param spec games.PerValueActions
---@return games.GameAction[]
function M.per_value(spec)
  return vim.tbl_map(function(value)
    local digits = tostring(value)
    ---@type games.GameAction
    return {
      key = digits,
      label = spec.label or spec.kind,
      action = { kind = spec.kind, [spec.field] = digits },
      available = function(update)
        return vim.tbl_contains(spec.legal(update.view), value)
      end,
    }
  end, spec.values)
end

---@class games.DirectionActions
---@field kind string the action's kind; the direction goes in its direction field
---@field arrows? boolean bind each arrow key too
---@field movable fun(view: table, direction: games.Direction): boolean

---An action per direction on h j k l, available while movable allows it.
---@param spec games.DirectionActions
---@return games.GameAction[]
function M.directions(spec)
  return vim.tbl_map(function(entry)
    ---@type games.GameAction
    return {
      key = entry.key,
      aliases = spec.arrows and { entry.arrow } or nil,
      label = entry.direction,
      action = { kind = spec.kind, direction = entry.direction },
      available = function(update)
        return spec.movable(update.view, entry.direction)
      end,
    }
  end, keys.HJKL)
end

return M
