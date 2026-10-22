local compose = require("games.ui.compose")
local highlights = require("games.ui.highlights")
local text_util = require("games.ui.text")

local Group = highlights.Group

local DEFAULT_KEY_HL = Group.MUTED
local PANEL_KEY_HL = Group.KEY
local PANEL_VALUE_HL = Group.LABEL

---@alias games.KvPair {[1]: string, [2]: string} {key, value}

---@class games.KvProps
---@field pairs games.KvPair[] in display order
---@field key_hl? games.Hl default GamesMuted
---@field value_hl? games.Hl default none

---@class games.KvWidget
---@field panel fun(pairs: games.KvPair[]): games.Node
---@overload fun(props: games.KvProps): games.Node

local M = {}

---@param props games.KvProps
---@return games.Node
local function render(props)
  local key_width = text_util.widest(props.pairs, function(pair)
    return pair[1]
  end)
  local lines = vim.tbl_map(function(pair)
    local key = text_util.pad_to_width(pair[1], key_width)
    return compose.row({
      compose.text(key, props.key_hl or DEFAULT_KEY_HL),
      compose.text(" "),
      compose.text(pair[2], props.value_hl),
    })
  end, props.pairs)
  return compose.col(lines)
end

---pairs as a side panel's legend: keys as keys, values as labels.
---@param pairs games.KvPair[]
---@return games.Node
function M.panel(pairs)
  return render({ pairs = pairs, key_hl = PANEL_KEY_HL, value_hl = PANEL_VALUE_HL })
end

---@type games.KvWidget
local widget = setmetatable(M, {
  __call = function(_, props)
    return render(props)
  end,
})
return widget
