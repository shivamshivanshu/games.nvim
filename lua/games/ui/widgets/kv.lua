local compose = require("games.ui.compose")
local text_util = require("games.ui.text")

---@alias games.KvPair {[1]: string, [2]: string} {key, value}

---@class games.KvProps
---@field pairs games.KvPair[] in display order

---@param props games.KvProps
---@return games.Node
return function(props)
  local key_width = text_util.max_display_width(vim.tbl_map(function(pair)
    return pair[1]
  end, props.pairs))
  local lines = vim.tbl_map(function(pair)
    local key = text_util.pad_to_width(pair[1], key_width)
    return compose.row({ compose.text(key, "GamesMuted"), compose.text(" " .. pair[2]) })
  end, props.pairs)
  return compose.col(lines)
end
