local compose = require("games.ui.compose")

---@class games.KvProps
---@field pairs {[1]: string, [2]: string}[] ordered {key, value} entries

---@param props games.KvProps
---@return games.Node
return function(props)
  local key_width = 0
  for _, pair in ipairs(props.pairs) do
    key_width = math.max(key_width, vim.fn.strdisplaywidth(pair[1]))
  end
  local lines = vim.tbl_map(function(pair)
    local key = pair[1] .. (" "):rep(key_width - vim.fn.strdisplaywidth(pair[1]))
    return compose.row({ compose.text(key, "GamesMuted"), compose.text(" " .. pair[2]) })
  end, props.pairs)
  return compose.col(lines)
end
