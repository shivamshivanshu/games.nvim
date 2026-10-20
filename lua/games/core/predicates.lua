local M = {}

---@param value any
---@return boolean
function M.is_positive_integer(value)
  return type(value) == "number" and value >= 1 and value == math.floor(value)
end

return M
