local compose = require("games.ui.compose")

local KEY_HL = "GamesKey"
local LABEL_HL = "GamesLabel"
local GAP = "  "

---@class games.KeyHint
---@field key string
---@field label string

---@class (private) games.KeyHintGroup
---@field keys string[]
---@field label string

---Adjacent hints that share a label, as one group each.
---@param hints games.KeyHint[]
---@return games.KeyHintGroup[]
local function grouped(hints)
  local groups = {}
  for _, hint in ipairs(hints) do
    local last = groups[#groups]
    if last and last.label == hint.label then
      last.keys[#last.keys + 1] = hint.key
    else
      groups[#groups + 1] = { keys = { hint.key }, label = hint.label }
    end
  end
  return groups
end

---@param a string
---@param b string
---@return boolean
local function follows(a, b)
  return #a == 1 and #b == 1 and b:byte() == a:byte() + 1
end

---Keys as written in one hint: runs of consecutive one-character keys become `first-last`.
---@param keys string[]
---@return string
local function key_text(keys)
  local parts, first = {}, 1
  for i = 1, #keys do
    if i == #keys or not follows(keys[i], keys[i + 1]) then
      parts[#parts + 1] = i > first and (keys[first] .. "-" .. keys[i]) or keys[i]
      first = i + 1
    end
  end
  return table.concat(parts, ",")
end

---One row of `key label` pairs, e.g. `m move  R resign  q close`. Adjacent hints with one
---label share it, e.g. `1-7 drop`.
---@param hints games.KeyHint[]
---@return games.Node
return function(hints)
  local parts = {}
  for i, group in ipairs(grouped(hints)) do
    if i > 1 then
      parts[#parts + 1] = compose.text(GAP)
    end
    parts[#parts + 1] = compose.text(key_text(group.keys), KEY_HL)
    parts[#parts + 1] = compose.text(" ")
    parts[#parts + 1] = compose.text(group.label, LABEL_HL)
  end
  return compose.row(parts)
end
