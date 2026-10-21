local compose = require("games.ui.compose")
local text_util = require("games.ui.text")

local DEFAULT_KEY_HL = "GamesMuted"
local PANEL_KEY_HL = "GamesKey"
local PANEL_VALUE_HL = "GamesLabel"

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
  local key_width = text_util.max_display_width(vim.tbl_map(function(pair)
    return pair[1]
  end, props.pairs))
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
