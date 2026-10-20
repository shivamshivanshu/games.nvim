local M = {}

---@param value any
---@return boolean
function M.is_positive_integer(value)
  return type(value) == "number" and value >= 1 and value == math.floor(value)
end

---Whether text holds a Unicode control character (Cc): C0, DEL or C1.
---@param text string UTF-8
---@return boolean
function M.has_control_character(text)
  return text:find("[%z\1-\31\127]") ~= nil or text:find("\194[\128-\159]") ~= nil
end

return M
