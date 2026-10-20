local compose = require("games.ui.compose")

local KEY_HL = "GamesKey"
local LABEL_HL = "GamesLabel"
local GAP = "  "

---@class games.KeyHint
---@field key string
---@field label string

---One row of `key label` pairs, e.g. `m move  R resign  q close`.
---@param hints games.KeyHint[]
---@return games.Node
return function(hints)
  local parts = {}
  for i, hint in ipairs(hints) do
    if i > 1 then
      parts[#parts + 1] = compose.text(GAP)
    end
    parts[#parts + 1] = compose.text(hint.key, KEY_HL)
    parts[#parts + 1] = compose.text(" ")
    parts[#parts + 1] = compose.text(hint.label, LABEL_HL)
  end
  return compose.row(parts)
end
