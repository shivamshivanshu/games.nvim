local M = {}

---@param kind string
---@param field string
---@return fun(text: string): table<string, string>
function M.text_field(kind, field)
  return function(text)
    return { kind = kind, [field] = text }
  end
end

return M
